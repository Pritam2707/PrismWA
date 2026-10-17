package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/store"
)

// -----------------------------------------------------------------------------
// SOCIAL MEDIA DOWNLOADER (Instagram, Facebook, Reddit, X, TikTok, ...)
//
// Usage:
//   !dl <url>            (aliases: !download !ig !insta !fb !facebook !reddit
//                          !tiktok !twitter !tw !threads !pin)
//   Reply to a message containing a link with !dl
//
// Backend: yt-dlp (all platforms) + Reddit .json fallback for image posts and
// galleries, which yt-dlp does not handle.
// -----------------------------------------------------------------------------

func RegisterSocialDownloadCommands(r *CommandRegistry) {
	for _, p := range []string{
		"!dl", "!download", "!ig", "!insta", "!instagram", "!fb", "!facebook",
		"!reddit", "!tiktok", "!twitter", "!tw", "!threads", "!pin",
	} {
		r.RegisterPrefix(p, handleSocialDownload)
	}
}

type socialSite struct {
	Name  string
	Emoji string
	Hosts []string
}

// Allowlist on purpose: yt-dlp's generic extractor will happily fetch any URL,
// including http://192.168.x.x or localhost, which anyone in a group could abuse.
var socialSites = []socialSite{
	{"Instagram", "📸", []string{"instagram.com", "instagr.am"}},
	{"Facebook", "📘", []string{"facebook.com", "fb.watch", "fb.com"}},
	{"Reddit", "👽", []string{"reddit.com", "redd.it"}},
	{"Twitter/X", "🐦", []string{"twitter.com", "x.com"}},
	{"TikTok", "🎵", []string{"tiktok.com"}},
	{"Threads", "🧵", []string{"threads.net", "threads.com"}},
	{"Pinterest", "📌", []string{"pinterest.com", "pin.it"}},
	{"Snapchat", "👻", []string{"snapchat.com"}},
	{"Twitch", "🎮", []string{"twitch.tv"}},
	{"Vimeo", "🎞️", []string{"vimeo.com"}},
	{"Streamable", "📼", []string{"streamable.com"}},
	{"Dailymotion", "📺", []string{"dailymotion.com", "dai.ly"}},
	{"SoundCloud", "🔊", []string{"soundcloud.com"}},
}

var (
	socialURLRe     = regexp.MustCompile(`https?://[^\s<>"']+`)
	socialVideoExts = map[string]bool{".mp4": true, ".mov": true, ".mkv": true, ".webm": true, ".m4v": true}
	socialImageExts = map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif"}
	socialAudioExts = map[string]string{".mp3": "audio/mpeg", ".m4a": "audio/mp4", ".ogg": "audio/ogg", ".opus": "audio/ogg", ".aac": "audio/aac"}
)

const (
	socialMaxFiles     = 10
	socialMaxSendBytes = 64 * 1024 * 1024
	socialUA           = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

func findSocialSite(host string) *socialSite {
	host = strings.ToLower(host)
	for i := range socialSites {
		for _, h := range socialSites[i].Hosts {
			if host == h || strings.HasSuffix(host, "."+h) {
				return &socialSites[i]
			}
		}
	}
	return nil
}

func socialSupportedList() string {
	var names []string
	for _, s := range socialSites {
		names = append(names, fmt.Sprintf("%s %s", s.Emoji, s.Name))
	}
	return strings.Join(names, "\n")
}

func extractSocialURL(c *CommandContext) string {
	clean := func(s string) string {
		return strings.TrimRight(s, ".,;:!?)]}>")
	}

	// 1. URL in the command itself
	if m := socialURLRe.FindString(c.RawText); m != "" {
		return clean(m)
	}

	// 2. URL in the quoted (replied-to) message
	if c.RawMsg == nil || c.RawMsg.Message == nil {
		return ""
	}
	quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
	if quoted == nil {
		return ""
	}
	candidates := []string{
		quoted.GetConversation(),
		quoted.GetExtendedTextMessage().GetText(),
		quoted.GetImageMessage().GetCaption(),
		quoted.GetVideoMessage().GetCaption(),
	}
	for _, txt := range candidates {
		if m := socialURLRe.FindString(txt); m != "" {
			return clean(m)
		}
	}
	return ""
}

func handleSocialDownload(c *CommandContext) (bool, error) {
	target := extractSocialURL(c)
	if target == "" {
		return true, c.ReplyText("📥 *Social Downloader*\n\nUsage: `!dl <link>` or reply to a message with a link using `!dl`\n\n*Supported:*\n" + socialSupportedList())
	}

	parsed, err := url.Parse(target)
	if err != nil || parsed.Host == "" {
		return true, c.ReplyText("❌ That doesn't look like a valid link.")
	}
	site := findSocialSite(parsed.Hostname())
	if site == nil {
		return true, c.ReplyText("❌ Unsupported site.\n\n*Supported:*\n" + socialSupportedList())
	}

	_ = c.React(site.Emoji)
	_ = c.ReplyText(fmt.Sprintf("%s *Fetching from %s...*", site.Emoji, site.Name))

	workDir, err := os.MkdirTemp("", "social-dl-*")
	if err != nil {
		return true, c.ReplyText("❌ Failed to create temp directory.")
	}
	defer os.RemoveAll(workDir)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	var files []string
	var ytErr error

	if _, lookErr := exec.LookPath("yt-dlp"); lookErr == nil {
		files, ytErr = runSocialYtDlp(ctx, workDir, target)
	} else {
		ytErr = fmt.Errorf("yt-dlp not installed")
	}

	// Reddit image posts / galleries are not supported by yt-dlp
	if len(files) == 0 && site.Name == "Reddit" {
		var rErr error
		files, rErr = redditFallback(ctx, workDir, target)
		if rErr != nil {
			logSocial("reddit fallback failed: %v", rErr)
		}
	}

	if len(files) == 0 {
		_ = c.React("❌")
		logSocial("download failed for %s: %v", target, ytErr)
		msg := fmt.Sprintf("❌ Couldn't download from %s.", site.Name)
		switch site.Name {
		case "Instagram", "Facebook":
			msg += "\n\nPossible reasons: private post, login wall, or missing/expired `cookies.txt` (export from a logged-in browser and put it next to the bot binary). Photo-only posts may also be unsupported."
		case "Reddit":
			msg += "\n\nPossible reasons: deleted post, NSFW/quarantined content, or Reddit blocking the server's IP."
		default:
			msg += "\n\nThe post may be private, deleted, or yt-dlp needs updating (`yt-dlp -U`)."
		}
		return true, c.ReplyText(msg)
	}

	if len(files) > socialMaxFiles {
		files = files[:socialMaxFiles]
	}

	caption := fmt.Sprintf("%s %s", site.Emoji, site.Name)
	sent := 0
	var lastErr error
	for _, f := range files {
		if err := sendSocialFile(ctx, c, f, caption); err != nil {
			lastErr = err
			logSocial("send failed for %s: %v", filepath.Base(f), err)
			continue
		}
		sent++
	}

	if sent == 0 {
		_ = c.React("❌")
		return true, c.ReplyText(fmt.Sprintf("❌ Downloaded, but sending failed: %v", lastErr))
	}
	_ = c.React("✅")
	return true, nil
}

func logSocial(format string, args ...interface{}) {
	fmt.Printf("[SOCIAL-DL] "+format+"\n", args...)
}

// -----------------------------------------------------------------------------
// yt-dlp backend
// -----------------------------------------------------------------------------

func runSocialYtDlp(ctx context.Context, dir, target string) ([]string, error) {
	args := []string{
		"--no-warnings",
		"--restrict-filenames",
		"--playlist-end", fmt.Sprint(socialMaxFiles), // carousels / multi-video posts
		"--max-filesize", "60M",
		"--concurrent-fragments", "4",
		"-f", "bv*[vcodec^=avc1][height<=720]+ba[acodec^=mp4a]/bv*[height<=720]+ba/b[height<=720]/bv*+ba/b",
		"--merge-output-format", "mp4",
		"-o", filepath.Join(dir, "%(autonumber)03d.%(ext)s"),
	}
	if _, err := exec.LookPath("node"); err == nil {
		args = append(args, "--js-runtimes", "node")
	}
	if _, err := os.Stat("cookies.txt"); err == nil {
		args = append(args, "--cookies", "cookies.txt")
	}
	args = append(args, target)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.CombinedOutput()

	files := collectSocialFiles(dir)
	if len(files) == 0 {
		if err == nil {
			err = fmt.Errorf("no output files")
		}
		return nil, fmt.Errorf("%w: %s", err, tailString(string(out), 400))
	}
	// Partial success (e.g. one carousel item failed) is still success.
	return files, nil
}

func collectSocialFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		_, isImg := socialImageExts[ext]
		_, isAud := socialAudioExts[ext]
		if socialVideoExts[ext] || isImg || isAud {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files
}

func tailString(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// -----------------------------------------------------------------------------
// Reddit fallback (images, galleries, crossposts) via public .json endpoint
// -----------------------------------------------------------------------------

type redditPost struct {
	Title               string `json:"title"`
	PostHint            string `json:"post_hint"`
	URLOverriddenByDest string `json:"url_overridden_by_dest"`
	IsGallery           bool   `json:"is_gallery"`
	GalleryData         struct {
		Items []struct {
			MediaID string `json:"media_id"`
		} `json:"items"`
	} `json:"gallery_data"`
	MediaMetadata map[string]struct {
		Status string `json:"status"`
		S      struct {
			U   string `json:"u"`
			Gif string `json:"gif"`
		} `json:"s"`
	} `json:"media_metadata"`
	CrosspostParentList []redditPost `json:"crosspost_parent_list"`
}

func redditFallback(ctx context.Context, dir, target string) ([]string, error) {
	client := &http.Client{Timeout: 20 * time.Second}

	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	// Resolve share links (/s/xxxx) and redd.it short links
	if strings.Contains(u.Path, "/s/") || strings.HasSuffix(strings.ToLower(u.Hostname()), "redd.it") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", socialUA)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
		u = resp.Request.URL
	}

	u.RawQuery = ""
	u.Fragment = ""
	jsonURL := strings.TrimRight(u.String(), "/") + ".json?raw_json=1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jsonURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", socialUA)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit json http %d", resp.StatusCode)
	}

	var listing []struct {
		Data struct {
			Children []struct {
				Data redditPost `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&listing); err != nil {
		return nil, err
	}
	if len(listing) == 0 || len(listing[0].Data.Children) == 0 {
		return nil, fmt.Errorf("empty reddit listing")
	}

	post := listing[0].Data.Children[0].Data
	if len(post.CrosspostParentList) > 0 {
		post = post.CrosspostParentList[0]
	}

	var imageURLs []string
	if post.IsGallery {
		for _, it := range post.GalleryData.Items {
			meta, ok := post.MediaMetadata[it.MediaID]
			if !ok || meta.Status != "valid" {
				continue
			}
			src := meta.S.U
			if src == "" {
				src = meta.S.Gif
			}
			if src != "" {
				imageURLs = append(imageURLs, src)
			}
		}
	} else if post.URLOverriddenByDest != "" {
		du, err := url.Parse(post.URLOverriddenByDest)
		if err == nil {
			_, isImgExt := socialImageExts[strings.ToLower(path.Ext(du.Path))]
			if post.PostHint == "image" || isImgExt {
				imageURLs = append(imageURLs, post.URLOverriddenByDest)
			}
		}
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("no image media in reddit post")
	}
	if len(imageURLs) > socialMaxFiles {
		imageURLs = imageURLs[:socialMaxFiles]
	}

	var files []string
	for i, src := range imageURLs {
		ext := ".jpg"
		if pu, err := url.Parse(src); err == nil {
			if _, ok := socialImageExts[strings.ToLower(path.Ext(pu.Path))]; ok {
				ext = strings.ToLower(path.Ext(pu.Path))
			}
		}
		dest := filepath.Join(dir, fmt.Sprintf("%03d%s", i+1, ext))
		if err := downloadSocialImage(ctx, client, src, dest, 25<<20); err != nil {
			logSocial("reddit image %d failed: %v", i+1, err)
			continue
		}
		files = append(files, dest)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("all reddit image downloads failed")
	}
	return files, nil
}

func downloadSocialImage(ctx context.Context, client *http.Client, src, dest string, maxBytes int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", socialUA)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("file too large")
	}
	if len(data) < 200 {
		return fmt.Errorf("file too small, likely not an image")
	}
	return os.WriteFile(dest, data, 0600)
}

// -----------------------------------------------------------------------------
// Sending
// -----------------------------------------------------------------------------

func sendSocialFile(ctx context.Context, c *CommandContext, file, caption string) error {
	ext := strings.ToLower(filepath.Ext(file))

	switch {
	case socialVideoExts[ext]:
		return sendSocialVideo(ctx, c, file, caption)

	case socialImageExts[ext] != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if len(data) > socialMaxSendBytes {
			return fmt.Errorf("image too large (%.1f MB)", float64(len(data))/(1024*1024))
		}
		uploaded, err := c.Client.Upload(context.Background(), data, whatsmeow.MediaImage)
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
		msg := &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(socialImageExts[ext]),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(data))),
				Caption:       proto.String(caption),
			},
		}
		resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		if err == nil && resp.ID != "" && store.GlobalPersistent != nil {
			_ = store.RecordBotMessage(resp.ID, c.ChatJID.String(), caption)
		}
		return err

	case socialAudioExts[ext] != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if len(data) > socialMaxSendBytes {
			return fmt.Errorf("audio too large (%.1f MB)", float64(len(data))/(1024*1024))
		}
		uploaded, err := c.Client.Upload(context.Background(), data, whatsmeow.MediaAudio)
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
		msg := &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(socialAudioExts[ext]),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(data))),
				PTT:           proto.Bool(false),
			},
		}
		resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		if err == nil && resp.ID != "" && store.GlobalPersistent != nil {
			_ = store.RecordBotMessage(resp.ID, c.ChatJID.String(), caption)
		}
		return err
	}

	return fmt.Errorf("unsupported file type %q", ext)
}

func sendSocialVideo(ctx context.Context, c *CommandContext, file, caption string) error {
	// Reuses helpers from media.go: re-encodes to H.264/AAC + faststart if needed
	compat, _ := ensureWhatsAppCompatibleVideo(ctx, file)
	if compat != file {
		defer os.Remove(compat)
	}

	stat, err := os.Stat(compat)
	if err != nil || stat.Size() == 0 {
		return fmt.Errorf("video file missing or empty")
	}
	if stat.Size() > socialMaxSendBytes {
		return fmt.Errorf("video too large for WhatsApp (%.1f MB)", float64(stat.Size())/(1024*1024))
	}

	thumb, _ := extractVideoThumbnail(ctx, compat)
	duration := getVideoDurationSeconds(ctx, compat)

	data, err := os.ReadFile(compat)
	if err != nil {
		return err
	}

	uploaded, err := c.Client.Upload(context.Background(), data, whatsmeow.MediaVideo)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	vm := &waProto.VideoMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String("video/mp4"),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
		Caption:       proto.String(caption),
	}
	if len(thumb) > 0 {
		vm.JPEGThumbnail = thumb
	}
	if duration > 0 {
		vm.Seconds = proto.Uint32(duration)
	}

	resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, &waProto.Message{VideoMessage: vm})
	if err == nil && resp.ID != "" && store.GlobalPersistent != nil {
		_ = store.RecordBotMessage(resp.ID, c.ChatJID.String(), caption)
	}
	return err
}
