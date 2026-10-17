package commands

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/gemini"
)

var voiceIDs = map[string]string{
	"zero2_jp":         "fe7fe6d5d7474c2e9c0eb778e1a28f18",
	"kurumi_en":        "09bb45f3522a4337b12f81971a94bbe8",
	"kurumi_jp":        "8023630490a7462a8d1ce3e72d9f85e0",
	"goku_en":          "a04b127b9c9448429a401a87a9b3d619",
	"elon_musk":        "5303cbf567be41a9894286277b051f83",
	"naruto_jp":        "087930b6a0d947b89c121294adb9b887",
	"naruto_en":        "0785ead631314bb794c6943599dbf24c",
	"paimon_en":        "a23bb0727f954c8d9d8cb6a57d63ae2d",
	"kratos":           "80d5909c6f50429f93e48f8b488f0650",
	"snoop_dog":        "8bc0ef3b96424e6db3cccf6360c69778",
	"eminem":           "9b1b1f8a52da4ee2bccc70c9c6c50182",
	"joshua_graham":    "c0dacdfe68454ea99fa42d56f04572ac",
	"rus_en":           "1fb0effbd4774ede894a13944da12245",
	"simon_riley":      "724c2151f1984a4e850c2210ca941000",
	"johan_en":         "e530f9e40efa4e238a9bf4a52e5d75fe",
	"light_en":         "605891dea98f424db46da9283be7b1b3",
	"l_en":             "a1a3247d2c4f4e8dbe4ba4479a36e7a5",
	"lara_croft":       "8099b8618bcf44268a488ab18fc3438c",
	"marin_en":         "a056bb7a7aca42e697f51142d9e70613",
	"rem_en":           "380f5c79a37a4ec39d3c0934ebc42222",
	"modi":             "8b734220ba20480aa95b5e6329730003",
	"trump":            "5196af35f6ff4a0dbf541793fc9f2157",
	"miku":             "14f2767336d140aaa048f37ee210a6f6",
	"sharukh_khan":     "6e00ff5a53f747d7ae7e2ef3e622a595",
	"amitabh_bachchan": "ea7cdc74aeae4b608be27fdc37fdcb05",
	"columnbina":       "f2aed07c91614db28daaaa849150cc6e",
	"sandrone":         "14cdaeb1098444a6a889a330dd7c6af5",
	"furina":           "bd08be872bc440918674af072944ba12",
	"venti":            "e34c486929524d41b88646b4ac2f382f",
	"zhongli":          "ed46a0f9790f4a38b702f4a66c6c112e",
	"mavuika":          "92096b35320744b6a37944b17d864f8c",
	"sreyan":           "9bf1cb0b988d4a22ad5c7790eca3037a",
}

func RegisterMediaCommands(r *CommandRegistry) {
	// View Once
	r.RegisterPrefix("!vo", handleViewOnce)
	r.RegisterPrefix("!viewonce", handleViewOnce)
	r.RegisterPrefix("!rv", handleViewOnce)
	r.RegisterPrefix("!resend", handleViewOnce)

	// Transcribe
	r.RegisterPrefix("!transcribe", handleTranscribe)
	r.RegisterPrefix("!analyze", handleTranscribe)

	// Image Search
	r.RegisterPrefix("!img", handleImageSearch)
	r.RegisterPrefix("!image", handleImageSearch)

	// AI Image Generation
	r.RegisterPrefix("!imagine", handleImagine)
	r.RegisterPrefix("!genimage", handleImagine)
	r.RegisterPrefix("!draw", handleImagine)
	r.RegisterPrefix("!paint", handleImagine)

	// TTS
	r.RegisterPrefix("!tts", handleTTS)
	r.RegisterPrefix("!voices", handleVoicesList)

	// Media Downloader (yt-dlp)
	r.RegisterPrefix("!play", handlePlayMusic)
	r.RegisterPrefix("!song", handlePlayMusic)
	r.RegisterPrefix("!video", handlePlayVideo)
	r.RegisterPrefix("!select", handleSelectMedia)
	r.RegisterPrefix("!pick", handleSelectMedia)

	r.Register(func(text string) bool {
		t := strings.TrimSpace(text)
		return len(t) == 1 && t[0] >= '1' && t[0] <= '5'
	}, handleNumberSelect)

	// Sticker & Media Conversion Tools
	r.RegisterPrefix("!sticker", handleSticker)
	r.RegisterPrefix("!s", handleSticker)
	r.RegisterPrefix("!toimg", handleToImage)
	r.RegisterPrefix("!togif", handleToGif)
	r.RegisterPrefix("!toaudio", handleToAudio)
	r.RegisterPrefix("!tomp3", handleToAudio)
	r.RegisterPrefix("!tovn", handleToAudio)

	// Social Media Downloader (Instagram, Facebook, Reddit, X, TikTok, etc.)
	RegisterSocialDownloadCommands(r)
}

// -----------------------------------------------------------------------------
// 1. VIEW ONCE REVEAL
// -----------------------------------------------------------------------------

func handleViewOnce(c *CommandContext) (bool, error) {
	quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
	var targetImg *waProto.ImageMessage
	var targetVid *waProto.VideoMessage
	var targetAud *waProto.AudioMessage

	if quoted != nil {
		if quoted.GetViewOnceMessage() != nil {
			inner := quoted.GetViewOnceMessage().GetMessage()
			targetImg = inner.GetImageMessage()
			targetVid = inner.GetVideoMessage()
			targetAud = inner.GetAudioMessage()
		} else if quoted.GetViewOnceMessageV2() != nil {
			inner := quoted.GetViewOnceMessageV2().GetMessage()
			targetImg = inner.GetImageMessage()
			targetVid = inner.GetVideoMessage()
			targetAud = inner.GetAudioMessage()
		} else {
			targetImg = quoted.GetImageMessage()
			targetVid = quoted.GetVideoMessage()
			targetAud = quoted.GetAudioMessage()
		}
	} else if c.RawMsg.Message.GetViewOnceMessage() != nil {
		inner := c.RawMsg.Message.GetViewOnceMessage().GetMessage()
		targetImg = inner.GetImageMessage()
		targetVid = inner.GetVideoMessage()
		targetAud = inner.GetAudioMessage()
	} else if c.RawMsg.Message.GetViewOnceMessageV2() != nil {
		inner := c.RawMsg.Message.GetViewOnceMessageV2().GetMessage()
		targetImg = inner.GetImageMessage()
		targetVid = inner.GetVideoMessage()
		targetAud = inner.GetAudioMessage()
	}

	if targetImg == nil && targetVid == nil && targetAud == nil {
		return true, c.ReplyText("Reply to a view-once photo, video, or voice note with !vo to reveal it.")
	}

	if targetImg != nil {
		data, err := c.Client.Download(c.Ctx, targetImg)
		if err != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to download view-once photo: %v", err))
		}
		uploaded, err := c.Client.Upload(context.Background(), data, whatsmeow.MediaImage)
		if err != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to upload photo: %v", err))
		}
		caption := targetImg.GetCaption()
		if caption == "" {
			caption = "Revealed photo"
		}
		_ = c.React("👁️")
		msg := &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      targetImg.Mimetype,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(data))),
				Caption:       proto.String(caption),
			},
		}
		_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		return true, err
	}

	if targetVid != nil {
		data, err := c.Client.Download(c.Ctx, targetVid)
		if err != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to download view-once video: %v", err))
		}
		uploaded, err := c.Client.Upload(context.Background(), data, whatsmeow.MediaVideo)
		if err != nil {
			return true, c.ReplyText(fmt.Sprintf("Failed to upload video: %v", err))
		}
		caption := targetVid.GetCaption()
		if caption == "" {
			caption = "Revealed video"
		}
		msg := &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      targetVid.Mimetype,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(data))),
				Caption:       proto.String(caption),
			},
		}
		_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
		return true, err
	}

	return true, c.ReplyText("Revealed view-once media.")
}

// -----------------------------------------------------------------------------
// 2. TRANSCRIBE & MULTIMODAL MEDIA ANALYSIS
// -----------------------------------------------------------------------------

func transcribeAudioGroq(ctx context.Context, audioData []byte, mimeType string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", errors.New("GROQ_API_KEY is not configured")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	filename := "audio.ogg"
	if strings.Contains(mimeType, "mp3") {
		filename = "audio.mp3"
	} else if strings.Contains(mimeType, "wav") {
		filename = "audio.wav"
	} else if strings.Contains(mimeType, "m4a") {
		filename = "audio.m4a"
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audioData); err != nil {
		return "", err
	}

	_ = writer.WriteField("model", "whisper-large-v3-turbo")
	_ = writer.WriteField("response_format", "json")
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.groq.com/openai/v1/audio/transcriptions", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 45 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq whisper error (%d): %s", resp.StatusCode, string(respBytes))
	}

	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return strings.TrimSpace(res.Text), nil
}

func handleTranscribe(c *CommandContext) (bool, error) {
	quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
	var audMsg *waProto.AudioMessage
	var imgMsg *waProto.ImageMessage

	if quoted != nil {
		audMsg = quoted.GetAudioMessage()
		imgMsg = quoted.GetImageMessage()
	} else {
		audMsg = c.RawMsg.Message.GetAudioMessage()
		imgMsg = c.RawMsg.Message.GetImageMessage()
	}

	if audMsg == nil && imgMsg == nil {
		return true, c.ReplyText("Reply to an audio, voice note, or photo with `!transcribe` to analyze.")
	}

	// Use independent background context with sufficient timeout
	dlCtx, dlCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer dlCancel()

	var data []byte
	var mimeType string
	var err error

	if audMsg != nil {
		_ = c.React("🎙️")
		_ = c.ReplyText("🎙️ *Transcribing voice note / audio... Please wait a moment.*")
		data, err = c.Client.Download(dlCtx, audMsg)
		mimeType = audMsg.GetMimetype()
		if mimeType == "" {
			mimeType = "audio/ogg"
		}
	} else {
		_ = c.React("🔍")
		_ = c.ReplyText("🔍 *Analyzing image... Please wait a moment.*")
		data, err = c.Client.Download(dlCtx, imgMsg)
		mimeType = imgMsg.GetMimetype()
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
	}

	if err != nil {
		_ = c.React("❌")
		return true, c.ReplyText(fmt.Sprintf("Failed to download media: %v", err))
	}

	prompt := "Transcribe or analyze the content of this media accurately and directly."
	parts := strings.Fields(c.RawText)
	if len(parts) > 1 {
		prompt = strings.Join(parts[1:], " ")
	}

	// 1. If audio and Groq is configured, try Groq Whisper first (ultra-fast < 1s)
	if audMsg != nil && os.Getenv("GROQ_API_KEY") != "" {
		whisperCtx, whisperCancel := context.WithTimeout(context.Background(), 45*time.Second)
		transcript, whisperErr := transcribeAudioGroq(whisperCtx, data, mimeType)
		whisperCancel()
		if whisperErr == nil && transcript != "" {
			_ = c.React("✅")
			return true, c.ReplyText(fmt.Sprintf("🎙️ *Audio Transcription (Whisper):*\n\n%s", transcript))
		}
		log.Printf("[Transcribe] Groq Whisper failed or skipped: %v, falling back to Gemini", whisperErr)
	}

	// 2. Multimodal analysis via Gemini with generous 90-second timeout
	b64 := base64.StdEncoding.EncodeToString(data)
	req := gemini.GenerateRequest{
		Contents: []gemini.Content{
			{
				Role: "user",
				Parts: []gemini.Part{
					{
						InlineData: &gemini.InlineData{
							MimeType: mimeType,
							Data:     b64,
						},
					},
					{Text: prompt},
				},
			},
		},
	}

	aiCtx, aiCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer aiCancel()

	result, err := gemini.GlobalClient.Generate(aiCtx, req)
	if err != nil {
		log.Printf("[MediaAnalysis] Error: %v", err)
		_ = c.React("❌")
		return true, c.ReplyText("⚠️ Media transcription/analysis failed. Please ensure the media is under 20MB and try again.")
	}

	_ = c.React("✅")
	header := "🔍 *Media Analysis:*"
	if audMsg != nil {
		header = "🎙️ *Audio Transcription:*"
	}
	return true, c.ReplyText(fmt.Sprintf("%s\n\n%s", header, strings.TrimSpace(result)))
}


// -----------------------------------------------------------------------------
// 3. IMAGE SEARCH (Google Serper API)
// -----------------------------------------------------------------------------

func handleImageSearch(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !img <search term>")
	}

	query := strings.Join(parts[1:], " ")
	_ = c.React("🔍")
	apiKey := os.Getenv("SERPER_API_KEY")
	if apiKey == "" {
		return true, c.ReplyText("SERPER_API_KEY is not configured in .env.")
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"q": query,
		"gl": "us",
		"hl": "en",
		"num": 3,
	})

	httpReq, err := http.NewRequestWithContext(c.Ctx, http.MethodPost, "https://google.serper.dev/images", bytes.NewReader(reqBody))
	if err != nil {
		return true, err
	}
	httpReq.Header.Set("X-API-KEY", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Image search request failed: %v", err))
	}
	defer resp.Body.Close()

	var serperResp struct {
		Images []struct {
			ImageURL string `json:"imageUrl"`
			Title    string `json:"title"`
		} `json:"images"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&serperResp); err != nil || len(serperResp.Images) == 0 {
		return true, c.ReplyText(fmt.Sprintf("No images found for \"%s\".", query))
	}

	firstImg := serperResp.Images[0]
	imgData, err := fetchRemoteFile(c.Ctx, firstImg.ImageURL)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Result found: %s\n%s", firstImg.Title, firstImg.ImageURL))
	}

	uploaded, err := c.Client.Upload(context.Background(), imgData, whatsmeow.MediaImage)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Result: %s", firstImg.ImageURL))
	}

	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("image/jpeg"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(imgData))),
			Caption:       proto.String(fmt.Sprintf("Image: %s", query)),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	return true, err
}

// -----------------------------------------------------------------------------
// 4. AI IMAGE GENERATION (Pollinations.ai FLUX / Turbo / Custom Fallbacks)
// -----------------------------------------------------------------------------

func generateImage(ctx context.Context, prompt string) ([]byte, string, string, error) {
	seed := time.Now().UnixNano() % 10000000
	encodedPrompt := strings.ReplaceAll(url.QueryEscape(prompt), "+", "%20")

	// Multi-tier Pollinations pipeline (Fast Sana, FLUX.1, and Turbo)
	tiers := []struct {
		name     string
		url      string
		timeout  time.Duration
	}{
		{
			name:    "Pollinations Fast",
			url:     fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=768&height=768&nologo=true&seed=%d", encodedPrompt, seed),
			timeout: 20 * time.Second,
		},
		{
			name:    "FLUX.1-schnell",
			url:     fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=1024&height=1024&model=flux&nologo=true&seed=%d", encodedPrompt, seed),
			timeout: 30 * time.Second,
		},
		{
			name:    "Turbo",
			url:     fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=768&height=768&model=turbo&nologo=true&seed=%d", encodedPrompt, seed),
			timeout: 25 * time.Second,
		},
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
	}

	for _, tier := range tiers {
		reqCtx, cancel := context.WithTimeout(ctx, tier.timeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, tier.url, nil)
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "image/jpeg,image/png,image/*;q=0.8")

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			log.Printf("[IMAGE-GEN] Tier %s connection error: %v", tier.name, err)
			continue
		}

		imgBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		log.Printf("[IMAGE-GEN] %s status: %d, len: %d, type: %s",
			tier.name, resp.StatusCode, len(imgBytes), resp.Header.Get("Content-Type"))

		if resp.StatusCode == http.StatusOK && len(imgBytes) > 1000 {
			mimetype := "image/jpeg"
			if bytes.HasPrefix(imgBytes, []byte("\x89PNG")) {
				mimetype = "image/png"
			} else if bytes.HasPrefix(imgBytes, []byte("RIFF")) && len(imgBytes) > 12 && string(imgBytes[8:12]) == "WEBP" {
				mimetype = "image/webp"
			}
			return imgBytes, mimetype, tier.name, nil
		}
	}

	// Fallback to custom Hugging Face endpoint if configured
	hfToken := os.Getenv("HUGGINGFACE_API_KEY")
	if hfToken == "" {
		hfToken = os.Getenv("HF_TOKEN")
	}
	if hfToken != "" {
		hfURL := os.Getenv("HUGGINGFACE_IMAGE_MODEL_URL")
		if hfURL != "" {
			body, _ := json.Marshal(map[string]string{"inputs": prompt})
			reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, hfURL, bytes.NewReader(body))
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+hfToken)
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{
					Transport: tr,
					Timeout:   45 * time.Second,
				}
				resp, err := client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						imgBytes, err := io.ReadAll(resp.Body)
						if err == nil && len(imgBytes) > 1000 {
							return imgBytes, "image/jpeg", "HuggingFace Custom", nil
						}
					}
				}
			}
		}
	}

	return nil, "", "", fmt.Errorf("all image generation providers failed")
}

func handleImagine(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("🎨 *Usage:* `!imagine <prompt>`\n_Example:_ `!imagine cyberpunk cat in neon tokyo`")
	}

	prompt := strings.TrimSpace(strings.Join(parts[1:], " "))
	if prompt == "" {
		return true, c.ReplyText("🎨 Please specify what image you'd like me to generate.")
	}

	// Immediate feedback with reaction & status message
	_ = c.React("🎨")
	_ = c.ReplyText(fmt.Sprintf("🎨 *Generating image...*\n> \"%s\"\n_Please wait a few seconds..._", prompt))

	// Insulate with independent background context (75 seconds)
	genCtx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	imgBytes, mimetype, modelName, err := generateImage(genCtx, prompt)
	if err != nil {
		_ = c.React("❌")
		log.Printf("[IMAGE-GEN] Failed to generate image for prompt %q: %v", prompt, err)
		return true, c.ReplyText("⚠️ Failed to generate image. Please try again with a different prompt.")
	}

	uploaded, err := c.Client.Upload(context.Background(), imgBytes, whatsmeow.MediaImage)
	if err != nil {
		_ = c.React("❌")
		log.Printf("[IMAGE-GEN] Failed to upload image to WhatsApp: %v", err)
		return true, c.ReplyText("⚠️ Failed to upload generated image to WhatsApp.")
	}

	caption := fmt.Sprintf("🎨 *Prompt:* %s\n✨ *Model:* %s", prompt, modelName)

	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimetype),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(imgBytes))),
			Caption:       proto.String(caption),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil {
		_ = c.React("✨")
	}
	return true, err
}



// -----------------------------------------------------------------------------
// 5. FISH AUDIO TTS
// -----------------------------------------------------------------------------

func getVoiceListFormatted() string {
	var voices []string
	for name := range voiceIDs {
		voices = append(voices, fmt.Sprintf("• `%s`", name))
	}
	sort.Strings(voices)
	return strings.Join(voices, "\n")
}

func handleTTS(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 3 {
		usage := fmt.Sprintf("🗣️ *TTS Usage:*\n`!tts <voicename> <text>`\n\n*Example:*\n`!tts elon_musk Hello world!`\n\n*Available Voices:*\n%s", getVoiceListFormatted())
		return true, c.ReplyText(usage)
	}

	voiceKey := strings.ToLower(parts[1])
	textToSpeak := strings.Join(parts[2:], " ")

	voiceID, isFishVoice := voiceIDs[voiceKey]
	if !isFishVoice && len(voiceKey) > 2 {
		errMsg := fmt.Sprintf("❌ Unknown voice name *\"%s\"*.\n\n*Available Voices:*\n%s", voiceKey, getVoiceListFormatted())
		return true, c.ReplyText(errMsg)
	}

	_ = c.React("🗣️")

	var audioBytes []byte
	isOpus := false

	// 1. Try Fish Audio using the free s2.1-pro-free model header
	apiKey := os.Getenv("FISH_AUDIO_API_KEY")
	if isFishVoice && apiKey != "" {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"text":         textToSpeak,
			"reference_id": voiceID,
			"format":       "mp3",
		})
		httpReq, err := http.NewRequestWithContext(c.Ctx, http.MethodPost, "https://api.fish.audio/v1/tts", bytes.NewReader(reqBody))
		if err == nil {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("model", "s2.1-pro-free")
			client := &http.Client{Timeout: 20 * time.Second}
			resp, reqErr := client.Do(httpReq)
			if reqErr == nil && resp.StatusCode == http.StatusOK {
				data, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr == nil && len(data) > 500 {
					audioBytes = data
				}
			} else if resp != nil {
				resp.Body.Close()
			}
		}
	}

	// 2. High-reliability Fallback: Google Translate TTS (supports en, ja, hi, fr, es, etc.)
	if len(audioBytes) == 0 {
		lang := "en"
		switch voiceKey {
		case "zero2_jp", "kurumi_jp", "naruto_jp", "ja", "jp", "japanese":
			lang = "ja"
		case "modi", "hi", "hindi":
			lang = "hi"
		case "fr", "french":
			lang = "fr"
		case "es", "spanish":
			lang = "es"
		case "de", "german":
			lang = "de"
		case "ru", "russian":
			lang = "ru"
		case "it", "italian":
			lang = "it"
		default:
			if len(voiceKey) == 2 {
				lang = voiceKey
			}
		}

		gURL := fmt.Sprintf("https://translate.google.com/translate_tts?ie=UTF-8&tl=%s&client=tw-ob&q=%s",
			lang, url.QueryEscape(textToSpeak))
		req, err := http.NewRequestWithContext(c.Ctx, http.MethodGet, gURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
			client := &http.Client{Timeout: 15 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				data, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr == nil && len(data) > 200 {
					audioBytes = data
				}
			} else if resp != nil {
				resp.Body.Close()
			}
		}
	}

	if len(audioBytes) == 0 {
		return true, c.ReplyText("❌ Failed to generate audio. Please check your `FISH_AUDIO_API_KEY` in .env.")
	}

	// 3. Convert MP3 to Ogg Opus using ffmpeg if available
	tmpDir := os.TempDir()
	inPath := filepath.Join(tmpDir, fmt.Sprintf("tts_in_%d.mp3", time.Now().UnixNano()))
	outPath := filepath.Join(tmpDir, fmt.Sprintf("tts_out_%d.ogg", time.Now().UnixNano()))

	if err := os.WriteFile(inPath, audioBytes, 0644); err == nil {
		defer os.Remove(inPath)
		cmd := exec.CommandContext(c.Ctx, "ffmpeg", "-y", "-i", inPath, "-c:a", "libopus", "-b:a", "64k", "-vn", outPath)
		if err := cmd.Run(); err == nil {
			if oggData, err := os.ReadFile(outPath); err == nil && len(oggData) > 200 {
				audioBytes = oggData
				isOpus = true
			}
			os.Remove(outPath)
		}
	}

	// 4. Upload and send message
	uploaded, err := c.Client.Upload(context.Background(), audioBytes, whatsmeow.MediaAudio)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to upload audio: %v", err))
	}

	mimetype := "audio/mpeg"
	ptt := false
	if isOpus {
		mimetype = "audio/ogg; codecs=opus"
		ptt = true
	}

	msg := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimetype),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(audioBytes))),
			PTT:           proto.Bool(ptt),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil {
		_ = c.React("✅")
	}
	return true, err
}

func handleVoicesList(c *CommandContext) (bool, error) {
	text := fmt.Sprintf("🎙️ *Available Voice Models*\n\n%s\n\n*Usage:* `!tts <voicename> <text>`", getVoiceListFormatted())
	return true, c.ReplyText(text)
}

// -----------------------------------------------------------------------------
// 6. MEDIA DOWNLOADER (yt-dlp wrapper)
// -----------------------------------------------------------------------------
// 6. MEDIA DOWNLOADER (yt-dlp wrapper with live download/upload progress)
// -----------------------------------------------------------------------------

func makeProgressBar(percent float64, length int) string {
	if length <= 0 {
		length = 10
	}
	filled := int((percent / 100.0) * float64(length))
	if filled < 0 {
		filled = 0
	}
	if filled > length {
		filled = length
	}
	empty := length - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func scanCRLFOrCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		if data[i] == '\r' || data[i] == '\n' {
			advance = i + 1
			if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				advance = i + 2
			}
			return advance, data[0:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

type SongSearchResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Duration string `json:"duration"`
	Channel  string `json:"channel"`
	URL      string `json:"url"`
}

type PendingMediaSearch struct {
	ChatJID   string
	SenderJID string
	Query     string
	Results   []SongSearchResult
	MsgID     string
	IsVideo   bool
	ExpiresAt time.Time
}

var (
	mediaSearchMu      sync.RWMutex
	pendingMediaSearch = make(map[string]*PendingMediaSearch)
	msgIdToMediaSearch = make(map[string]*PendingMediaSearch)
)

func getPendingSearch(c *CommandContext) *PendingMediaSearch {
	mediaSearchMu.Lock()
	defer mediaSearchMu.Unlock()

	// 1. Check if user quoted the search results message
	if c.RawMsg != nil && c.RawMsg.Message != nil {
		if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil {
			if ctxInfo := ext.GetContextInfo(); ctxInfo != nil && ctxInfo.StanzaID != nil {
				if s, ok := msgIdToMediaSearch[*ctxInfo.StanzaID]; ok {
					if time.Now().Before(s.ExpiresAt) {
						return s
					}
					delete(msgIdToMediaSearch, *ctxInfo.StanzaID)
				}
			}
		}
	}

	// 2. Check pending search for sender in this chat
	key := c.ChatJID.String() + ":" + c.SenderJID.String()
	if s, ok := pendingMediaSearch[key]; ok {
		if time.Now().Before(s.ExpiresAt) {
			return s
		}
		delete(pendingMediaSearch, key)
	}

	// 3. Fallback for DM chat where senderJID might equal chatJID
	if s, ok := pendingMediaSearch[c.ChatJID.String()]; ok {
		if time.Now().Before(s.ExpiresAt) {
			return s
		}
		delete(pendingMediaSearch, c.ChatJID.String())
	}

	return nil
}

func clearPendingSearch(c *CommandContext, s *PendingMediaSearch) {
	mediaSearchMu.Lock()
	defer mediaSearchMu.Unlock()

	if s != nil {
		if s.MsgID != "" {
			delete(msgIdToMediaSearch, s.MsgID)
		}
		delete(pendingMediaSearch, s.ChatJID+":"+s.SenderJID)
		delete(pendingMediaSearch, s.ChatJID)
	}
}

func searchYouTube(ctx context.Context, query string, limit int) ([]SongSearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	searchTarget := fmt.Sprintf("ytsearch%d:%s", limit, query)

	var args []string
	args = append(args, "--flat-playlist", "--no-warnings")
	if _, err := exec.LookPath("node"); err == nil {
		args = append(args, "--js-runtimes", "node")
	}
	if _, err := os.Stat("cookies.txt"); err == nil {
		args = append(args, "--cookies", "cookies.txt")
	}
	args = append(args,
		"--print", "%(id)s||%(title)s||%(duration_string)s||%(channel)s",
		searchTarget,
	)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var results []SongSearchResult
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "||")
		if len(parts) >= 2 {
			id := strings.TrimSpace(parts[0])
			title := strings.TrimSpace(parts[1])
			duration := "N/A"
			channel := "Unknown"
			if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" && strings.TrimSpace(parts[2]) != "NA" {
				duration = strings.TrimSpace(parts[2])
			}
			if len(parts) >= 4 && strings.TrimSpace(parts[3]) != "" && strings.TrimSpace(parts[3]) != "NA" {
				channel = strings.TrimSpace(parts[3])
			}
			results = append(results, SongSearchResult{
				ID:       id,
				Title:    title,
				Duration: duration,
				Channel:  channel,
				URL:      fmt.Sprintf("https://www.youtube.com/watch?v=%s", id),
			})
		}
	}
	return results, nil
}

func handleNumberSelect(c *CommandContext) (bool, error) {
	text := strings.TrimSpace(c.RawText)
	num, err := strconv.Atoi(text)
	if err != nil || num < 1 || num > 10 {
		return false, nil
	}

	search := getPendingSearch(c)
	if search == nil {
		return false, nil
	}

	if num > len(search.Results) {
		return true, c.ReplyText(fmt.Sprintf("Invalid choice. Please choose between 1 and %d.", len(search.Results)))
	}

	chosen := search.Results[num-1]
	clearPendingSearch(c, search)

	if search.IsVideo {
		return downloadAndSendVideo(c, chosen.URL, chosen.Title)
	}
	return downloadAndSendAudio(c, chosen.URL, chosen.Title)
}

func handleSelectMedia(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !select <number 1-5>")
	}
	num, err := strconv.Atoi(parts[1])
	if err != nil || num < 1 || num > 10 {
		return true, c.ReplyText("Invalid choice. Please select 1-5.")
	}

	search := getPendingSearch(c)
	if search == nil {
		return true, c.ReplyText("No active search found. Search first with `!play <song name>` or `!video <name>`.")
	}

	if num > len(search.Results) {
		return true, c.ReplyText(fmt.Sprintf("Invalid choice. Please select 1-%d.", len(search.Results)))
	}

	chosen := search.Results[num-1]
	clearPendingSearch(c, search)

	if search.IsVideo {
		return downloadAndSendVideo(c, chosen.URL, chosen.Title)
	}
	return downloadAndSendAudio(c, chosen.URL, chosen.Title)
}

func handlePlayMusic(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !play <song name or url>\nExample: !play blinding lights\nOr: !play https://youtu.be/...")
	}

	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return true, c.ReplyText("yt-dlp is not installed on this system. Install yt-dlp to use music features.")
	}

	// Check if this is a selection by number (e.g. !play 1)
	if len(parts) == 2 {
		if num, err := strconv.Atoi(parts[1]); err == nil && num >= 1 && num <= 10 {
			search := getPendingSearch(c)
			if search != nil {
				if num > len(search.Results) {
					return true, c.ReplyText(fmt.Sprintf("Invalid choice. Please select 1-%d.", len(search.Results)))
				}
				chosen := search.Results[num-1]
				clearPendingSearch(c, search)
				if search.IsVideo {
					return downloadAndSendVideo(c, chosen.URL, chosen.Title)
				}
				return downloadAndSendAudio(c, chosen.URL, chosen.Title)
			}
		}
	}

	query := strings.Join(parts[1:], " ")

	// If direct URL: download immediately without search menu
	if strings.HasPrefix(strings.ToLower(query), "http://") || strings.HasPrefix(strings.ToLower(query), "https://") {
		return downloadAndSendAudio(c, query, query)
	}

	// Search top 5 results
	_ = c.React("🔍")
	results, err := searchYouTube(c.Ctx, query, 5)
	if err != nil || len(results) == 0 {
		_ = c.React("❌")
		return true, c.ReplyText(fmt.Sprintf("❌ No search results found for: \"%s\"", query))
	}

	numberEmojis := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣"}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎵 *Song Search Results for:* _\"%s\"_\n\n", query))

	for i, item := range results {
		emoji := fmt.Sprintf("%d️⃣", i+1)
		if i < len(numberEmojis) {
			emoji = numberEmojis[i]
		}
		sb.WriteString(fmt.Sprintf("%s *%s*\n   ⏱️ `%s` • 👤 %s\n\n", emoji, item.Title, item.Duration, item.Channel))
	}
	sb.WriteString("👉 *Reply with a number (1-5)* or type `!play 1` to play!")

	sent, err := c.Client.SendMessage(c.Ctx, c.ChatJID, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(sb.String()),
		},
	})
	if err != nil {
		return true, err
	}

	// Store pending search
	mediaSearchMu.Lock()
	pending := &PendingMediaSearch{
		ChatJID:   c.ChatJID.String(),
		SenderJID: c.SenderJID.String(),
		Query:     query,
		Results:   results,
		MsgID:     sent.ID,
		IsVideo:   false,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	pendingMediaSearch[c.ChatJID.String()+":"+c.SenderJID.String()] = pending
	pendingMediaSearch[c.ChatJID.String()] = pending
	msgIdToMediaSearch[sent.ID] = pending
	mediaSearchMu.Unlock()

	return true, nil
}

func downloadAndSendAudio(c *CommandContext, target, displayTitle string) (bool, error) {
	_ = c.React("🔍")

	// Send initial status message
	statusResp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, &waProto.Message{
		Conversation: proto.String(fmt.Sprintf("⚡ *Searching and preparing audio:*\n\"%s\"...", displayTitle)),
	})
	var statusMsgID types.MessageID
	if err == nil {
		statusMsgID = statusResp.ID
	}

	var editMu sync.Mutex
	lastEdit := time.Now()
	updateStatus := func(text string, force bool) {
		if statusMsgID == "" {
			return
		}
		editMu.Lock()
		defer editMu.Unlock()
		if !force && time.Since(lastEdit) < 1500*time.Millisecond {
			return
		}
		lastEdit = time.Now()
		editMsg := c.Client.BuildEdit(c.ChatJID, statusMsgID, &waProto.Message{
			Conversation: proto.String(text),
		})
		_, _ = c.Client.SendMessage(context.Background(), c.ChatJID, editMsg)
	}

	now := time.Now().UnixNano()
	tmpPattern := filepath.Join(os.TempDir(), fmt.Sprintf("song-%d.%%(ext)s", now))

	if !strings.HasPrefix(target, "http") {
		target = "ytsearch1:" + target
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 120*time.Second)
	defer cancel()

	var args []string
	args = append(args, "--no-playlist", "--no-warnings")

	if _, err := exec.LookPath("node"); err == nil {
		args = append(args, "--js-runtimes", "node")
	}

	if _, err := os.Stat("cookies.txt"); err == nil {
		args = append(args, "--cookies", "cookies.txt")
	}

	args = append(args,
		"-f", "ba[ext=m4a]/ba/b",
		"--concurrent-fragments", "4",
		"-o", tmpPattern,
		"--max-filesize", "40M",
		target,
	)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		updateStatus(fmt.Sprintf("❌ Error initializing downloader: %v", err), true)
		return true, c.ReplyText("Downloader failed.")
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		updateStatus(fmt.Sprintf("❌ Failed to start download: %v", err), true)
		return true, c.ReplyText(fmt.Sprintf("Failed to run yt-dlp: %v", err))
	}

	rePercent := regexp.MustCompile(`\[download\]\s+([\d.]+)%`)
	reSpeed := regexp.MustCompile(`at\s+([\d.]+\s*\S+)`)
	reETA := regexp.MustCompile(`ETA\s+([\d:]+)`)

	scanner := bufio.NewScanner(stdout)
	scanner.Split(scanCRLFOrCR)

	for scanner.Scan() {
		line := scanner.Text()
		if pMatch := rePercent.FindStringSubmatch(line); len(pMatch) > 1 {
			var pct float64
			fmt.Sscanf(pMatch[1], "%f", &pct)
			speed := "N/A"
			if sMatch := reSpeed.FindStringSubmatch(line); len(sMatch) > 1 {
				speed = sMatch[1]
			}
			eta := "N/A"
			if eMatch := reETA.FindStringSubmatch(line); len(eMatch) > 1 {
				eta = eMatch[1]
			}
			bar := makeProgressBar(pct, 10)
			msgText := fmt.Sprintf("🎶 *%s*\n⬇️ *Downloading Audio...*\n\n`[%s]` *%.1f%%*\n🚀 *Speed:* %s | ⏱️ *ETA:* %s",
				displayTitle, bar, pct, speed, eta)
			updateStatus(msgText, false)
		}
	}
	_ = cmd.Wait()

	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), fmt.Sprintf("song-%d.*", now)))
	if len(matches) == 0 {
		updateStatus("❌ Download failed or file was not generated.", true)
		return true, c.ReplyText("Audio download failed.")
	}
	tmpFile := matches[0]
	defer os.Remove(tmpFile)

	stat, err := os.Stat(tmpFile)
	if err != nil || stat.Size() == 0 {
		updateStatus("❌ Download failed or file was not generated.", true)
		return true, c.ReplyText("Audio download failed.")
	}

	sizeMB := float64(stat.Size()) / (1024 * 1024)

	// Live Upload progress simulation
	stopUpload := make(chan struct{})
	go func() {
		pct := 20.0
		ticker := time.NewTicker(1200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopUpload:
				return
			case <-ticker.C:
				pct = math.Min(95.0, pct+25.0)
				bar := makeProgressBar(pct, 10)
				updateStatus(fmt.Sprintf("🎶 *%s*\n📤 *Uploading to WhatsApp... (%.1f MB)*\n\n`[%s]` *%.0f%%*",
					displayTitle, sizeMB, bar, pct), true)
			}
		}
	}()

	audioData, err := os.ReadFile(tmpFile)
	if err != nil {
		close(stopUpload)
		updateStatus("❌ Failed to read audio buffer.", true)
		return true, c.ReplyText("Failed to read audio file.")
	}

	uploaded, err := c.Client.Upload(context.Background(), audioData, whatsmeow.MediaAudio)
	close(stopUpload)

	if err != nil {
		updateStatus(fmt.Sprintf("❌ Failed to upload audio: %v", err), true)
		return true, c.ReplyText(fmt.Sprintf("Upload failed: %v", err))
	}

	updateStatus(fmt.Sprintf("✅ *Sent:* \"%s\" (%.1f MB)", displayTitle, sizeMB), true)

	mimeType := "audio/mp4"
	lowerFile := strings.ToLower(tmpFile)
	if strings.HasSuffix(lowerFile, ".mp3") {
		mimeType = "audio/mpeg"
	} else if strings.HasSuffix(lowerFile, ".ogg") || strings.HasSuffix(lowerFile, ".opus") {
		mimeType = "audio/ogg"
	}

	msg := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(audioData))),
			PTT:           proto.Bool(false),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil {
		_ = c.React("🎵")
	}
	return true, err
}

func handlePlayVideo(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !video <video name or url>\nExample: !video never gonna give you up\nOr: !video https://youtu.be/...")
	}

	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return true, c.ReplyText("yt-dlp is not installed on this system.")
	}

	// Check if this is a selection by number (e.g. !video 1)
	if len(parts) == 2 {
		if num, err := strconv.Atoi(parts[1]); err == nil && num >= 1 && num <= 10 {
			search := getPendingSearch(c)
			if search != nil {
				if num > len(search.Results) {
					return true, c.ReplyText(fmt.Sprintf("Invalid choice. Please select 1-%d.", len(search.Results)))
				}
				chosen := search.Results[num-1]
				clearPendingSearch(c, search)
				return downloadAndSendVideo(c, chosen.URL, chosen.Title)
			}
		}
	}

	query := strings.Join(parts[1:], " ")

	// If direct URL: download video immediately without search menu
	if strings.HasPrefix(strings.ToLower(query), "http://") || strings.HasPrefix(strings.ToLower(query), "https://") {
		return downloadAndSendVideo(c, query, query)
	}

	// Search top 5 results for video too
	_ = c.React("🔍")
	results, err := searchYouTube(c.Ctx, query, 5)
	if err != nil || len(results) == 0 {
		_ = c.React("❌")
		return true, c.ReplyText(fmt.Sprintf("❌ No search results found for: \"%s\"", query))
	}

	numberEmojis := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣"}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎬 *Video Search Results for:* _\"%s\"_\n\n", query))

	for i, item := range results {
		emoji := fmt.Sprintf("%d️⃣", i+1)
		if i < len(numberEmojis) {
			emoji = numberEmojis[i]
		}
		sb.WriteString(fmt.Sprintf("%s *%s*\n   ⏱️ `%s` • 👤 %s\n\n", emoji, item.Title, item.Duration, item.Channel))
	}
	sb.WriteString("👉 *Reply with a number (1-5)* or type `!video 1` to download video!")

	sent, err := c.Client.SendMessage(c.Ctx, c.ChatJID, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(sb.String()),
		},
	})
	if err != nil {
		return true, err
	}

	mediaSearchMu.Lock()
	pending := &PendingMediaSearch{
		ChatJID:   c.ChatJID.String(),
		SenderJID: c.SenderJID.String(),
		Query:     query,
		Results:   results,
		MsgID:     sent.ID,
		IsVideo:   true,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	pendingMediaSearch[c.ChatJID.String()+":"+c.SenderJID.String()] = pending
	pendingMediaSearch[c.ChatJID.String()] = pending
	msgIdToMediaSearch[sent.ID] = pending
	mediaSearchMu.Unlock()

	return true, nil
}

func downloadAndSendVideo(c *CommandContext, target, displayTitle string) (bool, error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return true, c.ReplyText("yt-dlp is not installed on this system.")
	}

	_ = c.React("🎬")

	// Send initial status message
	statusResp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, &waProto.Message{
		Conversation: proto.String(fmt.Sprintf("⚡ *Searching and preparing video:*\n\"%s\"...", displayTitle)),
	})
	var statusMsgID types.MessageID
	if err == nil {
		statusMsgID = statusResp.ID
	}

	var editMu sync.Mutex
	lastEdit := time.Now()
	updateStatus := func(text string, force bool) {
		if statusMsgID == "" {
			return
		}
		editMu.Lock()
		defer editMu.Unlock()
		if !force && time.Since(lastEdit) < 1500*time.Millisecond {
			return
		}
		lastEdit = time.Now()
		editMsg := c.Client.BuildEdit(c.ChatJID, statusMsgID, &waProto.Message{
			Conversation: proto.String(text),
		})
		_, _ = c.Client.SendMessage(context.Background(), c.ChatJID, editMsg)
	}

	now := time.Now().UnixNano()
	tmpPattern := filepath.Join(os.TempDir(), fmt.Sprintf("vid-%d.%%(ext)s", now))

	if !strings.HasPrefix(target, "http") {
		target = "ytsearch1:" + target
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 180*time.Second)
	defer cancel()

	var args []string
	args = append(args, "--no-playlist", "--no-warnings")

	if _, err := exec.LookPath("node"); err == nil {
		args = append(args, "--js-runtimes", "node")
	}

	if _, err := os.Stat("cookies.txt"); err == nil {
		args = append(args, "--cookies", "cookies.txt")
	}

	// Prefer native H.264 (avc1) video and AAC (mp4a) audio <= 720p; fallback to any stream
	args = append(args,
		"-f", "bv*[vcodec^=avc1][height<=720]+ba[acodec^=mp4a]/b[ext=mp4][height<=720]/bv*[height<=720]+ba/b[height<=720]/bv*+ba/b",
		"--merge-output-format", "mp4",
		"--concurrent-fragments", "4",
		"-o", tmpPattern,
		"--max-filesize", "45M",
		target,
	)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		updateStatus(fmt.Sprintf("❌ Error initializing video downloader: %v", err), true)
		return true, c.ReplyText("Downloader failed.")
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		updateStatus(fmt.Sprintf("❌ Failed to start video download: %v", err), true)
		return true, c.ReplyText(fmt.Sprintf("Failed to run yt-dlp: %v", err))
	}

	rePercent := regexp.MustCompile(`\[download\]\s+([\d.]+)%`)
	reSpeed := regexp.MustCompile(`at\s+([\d.]+\s*\S+)`)
	reETA := regexp.MustCompile(`ETA\s+([\d:]+)`)

	scanner := bufio.NewScanner(stdout)
	scanner.Split(scanCRLFOrCR)

	for scanner.Scan() {
		line := scanner.Text()
		if pMatch := rePercent.FindStringSubmatch(line); len(pMatch) > 1 {
			var pct float64
			fmt.Sscanf(pMatch[1], "%f", &pct)
			speed := "N/A"
			if sMatch := reSpeed.FindStringSubmatch(line); len(sMatch) > 1 {
				speed = sMatch[1]
			}
			eta := "N/A"
			if eMatch := reETA.FindStringSubmatch(line); len(eMatch) > 1 {
				eta = eMatch[1]
			}
			bar := makeProgressBar(pct, 10)
			msgText := fmt.Sprintf("🎬 *%s*\n⬇️ *Downloading Video...*\n\n`[%s]` *%.1f%%*\n🚀 *Speed:* %s | ⏱️ *ETA:* %s",
				displayTitle, bar, pct, speed, eta)
			updateStatus(msgText, false)
		}
	}
	_ = cmd.Wait()

	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), fmt.Sprintf("vid-%d.*", now)))
	if len(matches) == 0 {
		updateStatus("❌ Video download failed or format was unavailable.", true)
		return true, c.ReplyText("Video download failed.")
	}
	tmpFile := matches[0]
	defer os.Remove(tmpFile)

	stat, err := os.Stat(tmpFile)
	if err != nil || stat.Size() == 0 {
		updateStatus("❌ Video file was empty or not generated.", true)
		return true, c.ReplyText("Video download failed.")
	}

	// Transcode or remux to ensure 100% WhatsApp-compatible H.264 (yuv420p) + AAC + faststart
	updateStatus(fmt.Sprintf("🎬 *%s*\n⚙️ *Optimizing video for WhatsApp playback...*", displayTitle), true)
	compatibleFile, _ := ensureWhatsAppCompatibleVideo(ctx, tmpFile)
	if compatibleFile != tmpFile {
		defer os.Remove(compatibleFile)
	}

	cStat, err := os.Stat(compatibleFile)
	if err == nil && cStat.Size() > 0 {
		stat = cStat
		tmpFile = compatibleFile
	}

	thumbBytes, _ := extractVideoThumbnail(ctx, tmpFile)
	durationSec := getVideoDurationSeconds(ctx, tmpFile)

	sizeMB := float64(stat.Size()) / (1024 * 1024)

	// Live Upload progress simulation
	stopUpload := make(chan struct{})
	go func() {
		pct := 20.0
		ticker := time.NewTicker(1200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopUpload:
				return
			case <-ticker.C:
				pct = math.Min(95.0, pct+25.0)
				bar := makeProgressBar(pct, 10)
				updateStatus(fmt.Sprintf("🎬 *%s*\n📤 *Uploading to WhatsApp... (%.1f MB)*\n\n`[%s]` *%.0f%%*",
					displayTitle, sizeMB, bar, pct), true)
			}
		}
	}()

	vidData, err := os.ReadFile(tmpFile)
	if err != nil {
		close(stopUpload)
		updateStatus("❌ Failed to read video buffer.", true)
		return true, c.ReplyText("Failed to read video file.")
	}

	uploaded, err := c.Client.Upload(context.Background(), vidData, whatsmeow.MediaVideo)
	close(stopUpload)

	if err != nil {
		updateStatus(fmt.Sprintf("❌ Failed to upload video: %v", err), true)
		return true, c.ReplyText(fmt.Sprintf("Upload failed: %v", err))
	}

	updateStatus(fmt.Sprintf("✅ *Sent:* \"%s\" (%.1f MB)", displayTitle, sizeMB), true)

	videoMsg := &waProto.VideoMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String("video/mp4"),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(vidData))),
		Caption:       proto.String(displayTitle),
	}
	if len(thumbBytes) > 0 {
		videoMsg.JPEGThumbnail = thumbBytes
	}
	if durationSec > 0 {
		videoMsg.Seconds = proto.Uint32(durationSec)
	}

	msg := &waProto.Message{
		VideoMessage: videoMsg,
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil {
		_ = c.React("🎬")
	}
	return true, err
}

func probeVideoCodecs(ctx context.Context, filePath string) (vcodec, acodec string, err error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return "", "", err
	}
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-show_entries", "stream=codec_type,codec_name",
		"-of", "csv=p=0", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		parts := strings.Split(strings.TrimSpace(l), ",")
		if len(parts) >= 2 {
			col1 := strings.ToLower(strings.TrimSpace(parts[0]))
			col2 := strings.ToLower(strings.TrimSpace(parts[1]))
			if col2 == "video" && vcodec == "" {
				vcodec = col1
			} else if col1 == "video" && vcodec == "" {
				vcodec = col2
			} else if col2 == "audio" && acodec == "" {
				acodec = col1
			} else if col1 == "audio" && acodec == "" {
				acodec = col2
			}
		}
	}
	return vcodec, acodec, nil
}


func extractVideoThumbnail(ctx context.Context, videoPath string) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, err
	}
	thumbPath := filepath.Join(os.TempDir(), fmt.Sprintf("thumb-%d.jpg", time.Now().UnixNano()))
	defer os.Remove(thumbPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "00:00:01", "-i", videoPath, "-vframes", "1", "-vf", "scale=320:-1", "-q:v", "3", thumbPath)
	if err := cmd.Run(); err != nil {
		cmdZero := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "00:00:00", "-i", videoPath, "-vframes", "1", "-vf", "scale=320:-1", "-q:v", "3", thumbPath)
		if errZero := cmdZero.Run(); errZero != nil {
			return nil, err
		}
	}
	return os.ReadFile(thumbPath)
}

func getVideoDurationSeconds(ctx context.Context, videoPath string) uint32 {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return 0
	}
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var sec float64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &sec)
	if sec > 0 {
		return uint32(sec)
	}
	return 0
}

func ensureWhatsAppCompatibleVideo(ctx context.Context, inputPath string) (string, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return inputPath, nil
	}

	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("wa-comp-%d.mp4", time.Now().UnixNano()))

	vcodec, acodec, err := probeVideoCodecs(ctx, inputPath)
	isH264 := strings.EqualFold(vcodec, "h264") || strings.EqualFold(vcodec, "avc1")
	isAAC := strings.EqualFold(acodec, "aac") || strings.EqualFold(acodec, "mp4a")
	ext := strings.ToLower(filepath.Ext(inputPath))

	// If already an MP4 with H.264 video and AAC audio, fast remux to guarantee +faststart (moov atom at front)
	if err == nil && isH264 && isAAC && ext == ".mp4" {
		cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputPath,
			"-c", "copy",
			"-movflags", "+faststart",
			outputPath)
		if err := cmd.Run(); err == nil {
			if stat, err := os.Stat(outputPath); err == nil && stat.Size() > 0 {
				return outputPath, nil
			}
		}
	}

	// Full transcode to 100% WhatsApp-compatible H.264 (yuv420p) + AAC stereo + faststart
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputPath,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-preset", "veryfast",
		"-crf", "24",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "44100",
		"-ac", "2",
		"-movflags", "+faststart",
		outputPath)
	if err := cmd.Run(); err != nil {
		log.Printf("[VIDEO] ffmpeg transcode error: %v, falling back to input", err)
		return inputPath, nil
	}

	if stat, err := os.Stat(outputPath); err == nil && stat.Size() > 0 {
		return outputPath, nil
	}

	return inputPath, nil
}



// -----------------------------------------------------------------------------
// 7. STICKER TOOLS (ffmpeg)
// -----------------------------------------------------------------------------

func handleSticker(c *CommandContext) (bool, error) {
	quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
	var imgMsg *waProto.ImageMessage

	if quoted != nil {
		imgMsg = quoted.GetImageMessage()
	} else {
		imgMsg = c.RawMsg.Message.GetImageMessage()
	}

	if imgMsg == nil {
		return true, c.ReplyText("Send or reply to an image with !s to create a sticker.")
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return true, c.ReplyText("ffmpeg is required on the system for sticker conversion.")
	}

	data, err := c.Client.Download(c.Ctx, imgMsg)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to download image: %v", err))
	}

	tmpIn := filepath.Join(os.TempDir(), fmt.Sprintf("in-%d.jpg", time.Now().UnixNano()))
	tmpOut := filepath.Join(os.TempDir(), fmt.Sprintf("out-%d.webp", time.Now().UnixNano()))
	defer os.Remove(tmpIn)
	defer os.Remove(tmpOut)

	if err := os.WriteFile(tmpIn, data, 0644); err != nil {
		return true, err
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", tmpIn, "-vf", "scale=512:512:force_original_aspect_ratio=decrease,fps=15,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=0x00000000", "-c:v", "libwebp", "-quality", "75", tmpOut)
	if out, err := cmd.CombinedOutput(); err != nil {
		return true, c.ReplyText(fmt.Sprintf("ffmpeg conversion failed: %v", string(out)))
	}

	webpBytes, err := os.ReadFile(tmpOut)
	if err != nil {
		return true, c.ReplyText("Failed to read converted sticker.")
	}

	uploaded, err := c.Client.Upload(context.Background(), webpBytes, whatsmeow.MediaImage)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to upload sticker: %v", err))
	}

	msg := &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("image/webp"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(webpBytes))),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	return true, err
}

func handleToImage(c *CommandContext) (bool, error) {
	quoted := c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
	if quoted == nil || quoted.GetStickerMessage() == nil {
		return true, c.ReplyText("Reply to a sticker with !toimg to convert it to an image.")
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return true, c.ReplyText("ffmpeg is required on the system for image conversion.")
	}

	stkMsg := quoted.GetStickerMessage()
	data, err := c.Client.Download(c.Ctx, stkMsg)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to download sticker: %v", err))
	}

	tmpIn := filepath.Join(os.TempDir(), fmt.Sprintf("stk-%d.webp", time.Now().UnixNano()))
	tmpOut := filepath.Join(os.TempDir(), fmt.Sprintf("stk-%d.png", time.Now().UnixNano()))
	defer os.Remove(tmpIn)
	defer os.Remove(tmpOut)

	if err := os.WriteFile(tmpIn, data, 0644); err != nil {
		return true, err
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 30*time.Second)
	defer cancel()

	if err := convertWebpToPng(ctx, tmpIn, tmpOut); err != nil {
		return true, c.ReplyText(fmt.Sprintf("Conversion failed: %v", err))
	}

	pngBytes, err := os.ReadFile(tmpOut)
	if err != nil {
		return true, c.ReplyText("Failed to read converted image.")
	}

	uploaded, err := c.Client.Upload(context.Background(), pngBytes, whatsmeow.MediaImage)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to upload image: %v", err))
	}

	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("image/png"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(pngBytes))),
			Caption:       proto.String("Converted sticker"),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	return true, err
}

func convertWebpToPng(ctx context.Context, webpPath, pngPath string) error {
	// 1. Try direct ffmpeg first
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", webpPath, pngPath)
	if _, err := cmd.CombinedOutput(); err == nil {
		if fi, sErr := os.Stat(pngPath); sErr == nil && fi.Size() > 0 {
			return nil
		}
	}

	// 2. Try Python Pillow for animated or complex WebP
	for _, pyBin := range []string{"python", "python3"} {
		pyCmd := exec.CommandContext(ctx, pyBin, "-c",
			"import sys; from PIL import Image; im=Image.open(sys.argv[1]); im.convert('RGBA').save(sys.argv[2])",
			webpPath, pngPath)
		if err := pyCmd.Run(); err == nil {
			if fi, err := os.Stat(pngPath); err == nil && fi.Size() > 0 {
				return nil
			}
		}
	}

	// 3. Try Node Sharp
	nodeCmd := exec.CommandContext(ctx, "node", "-e",
		"const sharp = require('sharp'); sharp(process.argv[1]).png().toFile(process.argv[2]).then(() => process.exit(0)).catch(() => process.exit(1));",
		webpPath, pngPath)
	if err := nodeCmd.Run(); err == nil {
		if fi, err := os.Stat(pngPath); err == nil && fi.Size() > 0 {
			return nil
		}
	}

	return fmt.Errorf("failed to convert webp to png image")
}

func convertWebpToMp4(ctx context.Context, webpPath, mp4Path string) error {
	// 1. Try direct ffmpeg first (works for static WebP or FFmpeg builds with animated webp demuxers)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", webpPath, "-movflags", "faststart", "-pix_fmt", "yuv420p", "-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2", mp4Path)
	if _, err := cmd.CombinedOutput(); err == nil {
		if fi, sErr := os.Stat(mp4Path); sErr == nil && fi.Size() > 0 {
			return nil
		}
	}

	// 2. Animated WebP: Use Python Pillow to extract clean PNG frames and assemble MP4 directly via FFmpeg
	pyScript := `import sys, os, tempfile, shutil, subprocess
from PIL import Image, ImageSequence

src = sys.argv[1]
dst = sys.argv[2]
im = Image.open(src)

td = tempfile.mkdtemp(prefix='stk-frames-')
try:
    durations = []
    idx = 0
    for frame in ImageSequence.Iterator(im):
        frame.convert('RGBA').save(os.path.join(td, f'{idx:04d}.png'))
        durations.append(frame.info.get('duration', 40))
        idx += 1

    avg_dur = sum(durations) / len(durations) if durations else 40
    fps = max(1, min(60, round(1000 / avg_dur))) if avg_dur > 0 else 25

    cmd = [
        'ffmpeg', '-y',
        '-framerate', str(fps),
        '-i', os.path.join(td, '%04d.png'),
        '-movflags', 'faststart',
        '-pix_fmt', 'yuv420p',
        '-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2',
        dst
    ]
    res = subprocess.run(cmd, capture_output=True)
    if res.returncode != 0:
        sys.exit(res.returncode)
finally:
    shutil.rmtree(td, ignore_errors=True)
`
	for _, pyBin := range []string{"python", "python3"} {
		pyCmd := exec.CommandContext(ctx, pyBin, "-c", pyScript, webpPath, mp4Path)
		if err := pyCmd.Run(); err == nil {
			if fi, err := os.Stat(mp4Path); err == nil && fi.Size() > 0 {
				return nil
			}
		}
	}

	// 3. Fallback: Use Node.js with Sharp
	nodeScript := `const sharp = require('sharp');
const fs = require('fs');
const path = require('path');
const os = require('os');
const { execFileSync } = require('child_process');

async function main() {
  const src = process.argv[1];
  const dst = process.argv[2];
  const td = fs.mkdtempSync(path.join(os.tmpdir(), 'stk-sharp-'));
  try {
    const meta = await sharp(src, { animated: true }).metadata();
    const pages = meta.pages || 1;
    const delay = (meta.delay && meta.delay[0]) || 40;
    const fps = Math.max(1, Math.min(60, Math.round(1000 / delay)));

    for (let i = 0; i < pages; i++) {
      await sharp(src, { page: i }).png().toFile(path.join(td, String(i).padStart(4, '0') + '.png'));
    }

    execFileSync('ffmpeg', [
      '-y',
      '-framerate', String(fps),
      '-i', path.join(td, '%04d.png'),
      '-movflags', 'faststart',
      '-pix_fmt', 'yuv420p',
      '-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2',
      dst
    ]);
  } finally {
    fs.rmSync(td, { recursive: true, force: true });
  }
}
main().then(() => process.exit(0)).catch(() => process.exit(1));
`
	nodeCmd := exec.CommandContext(ctx, "node", "-e", nodeScript, webpPath, mp4Path)
	if err := nodeCmd.Run(); err == nil {
		if fi, err := os.Stat(mp4Path); err == nil && fi.Size() > 0 {
			return nil
		}
	}

	return fmt.Errorf("failed to convert animated sticker to MP4 (requires Python with Pillow or Node with Sharp)")
}

func fetchRemoteFile(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid url scheme")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func handleToGif(c *CommandContext) (bool, error) {
	var stickerMsg *waProto.StickerMessage
	if c.RawMsg.Message.GetStickerMessage() != nil {
		stickerMsg = c.RawMsg.Message.GetStickerMessage()
	} else if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil && ext.GetContextInfo() != nil {
		if q := ext.GetContextInfo().GetQuotedMessage(); q != nil {
			stickerMsg = q.GetStickerMessage()
		}
	}

	if stickerMsg == nil {
		return true, c.ReplyText("Reply to an animated sticker with `!togif` to convert it to GIF.")
	}

	_ = c.React("⏳")
	data, err := c.Client.Download(c.Ctx, stickerMsg)
	if err != nil || len(data) == 0 {
		return true, c.ReplyText("Failed to download sticker.")
	}

	now := time.Now().UnixNano()
	tmpWebp := filepath.Join(os.TempDir(), fmt.Sprintf("stk-%d.webp", now))
	tmpMp4 := filepath.Join(os.TempDir(), fmt.Sprintf("stk-%d.mp4", now))
	defer os.Remove(tmpWebp)
	defer os.Remove(tmpMp4)

	if err := os.WriteFile(tmpWebp, data, 0600); err != nil {
		return true, c.ReplyText("Failed to save temp sticker file.")
	}

	ctx, cancel := context.WithTimeout(c.Ctx, 45*time.Second)
	defer cancel()

	if err := convertWebpToMp4(ctx, tmpWebp, tmpMp4); err != nil {
		return true, c.ReplyText(fmt.Sprintf("GIF conversion failed: %v", err))
	}

	mp4Bytes, err := os.ReadFile(tmpMp4)
	if err != nil || len(mp4Bytes) == 0 {
		return true, c.ReplyText("Failed to read converted GIF.")
	}

	uploaded, err := c.Client.Upload(context.Background(), mp4Bytes, whatsmeow.MediaVideo)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to upload GIF: %v", err))
	}

	msg := &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("video/mp4"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mp4Bytes))),
			GifPlayback:   proto.Bool(true),
			Caption:       proto.String("Converted GIF"),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil {
		_ = c.React("✅")
	}
	return true, err
}

func handleToAudio(c *CommandContext) (bool, error) {
	var targetVideo *waProto.VideoMessage
	var targetAudio *waProto.AudioMessage

	if c.RawMsg.Message.GetVideoMessage() != nil {
		targetVideo = c.RawMsg.Message.GetVideoMessage()
	} else if c.RawMsg.Message.GetAudioMessage() != nil {
		targetAudio = c.RawMsg.Message.GetAudioMessage()
	} else if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil && ext.GetContextInfo() != nil {
		if q := ext.GetContextInfo().GetQuotedMessage(); q != nil {
			targetVideo = q.GetVideoMessage()
			targetAudio = q.GetAudioMessage()
		}
	}

	if targetVideo == nil && targetAudio == nil {
		return true, c.ReplyText("Reply to a video or audio note with `!toaudio` or `!tomp3` to extract audio.")
	}

	_ = c.React("⏳")
	var data []byte
	var err error
	if targetVideo != nil {
		data, err = c.Client.Download(c.Ctx, targetVideo)
	} else {
		data, err = c.Client.Download(c.Ctx, targetAudio)
	}

	if err != nil || len(data) == 0 {
		return true, c.ReplyText("Failed to download media file.")
	}

	now := time.Now().UnixNano()
	tmpIn := filepath.Join(os.TempDir(), fmt.Sprintf("media-%d.bin", now))
	tmpMp3 := filepath.Join(os.TempDir(), fmt.Sprintf("media-%d.mp3", now))
	defer os.Remove(tmpIn)
	defer os.Remove(tmpMp3)

	if err := os.WriteFile(tmpIn, data, 0600); err != nil {
		return true, c.ReplyText("Failed to save temp media.")
	}

	cmd := exec.CommandContext(c.Ctx, "ffmpeg", "-y", "-i", tmpIn, "-vn", "-acodec", "libmp3lame", "-q:a", "2", tmpMp3)
	if out, err := cmd.CombinedOutput(); err != nil {
		return true, c.ReplyText(fmt.Sprintf("Audio extraction failed: %v (%s)", err, string(out)))
	}

	mp3Bytes, err := os.ReadFile(tmpMp3)
	if err != nil || len(mp3Bytes) == 0 {
		return true, c.ReplyText("Failed to read converted audio.")
	}

	uploaded, err := c.Client.Upload(context.Background(), mp3Bytes, whatsmeow.MediaAudio)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to upload audio: %v", err))
	}

	msg := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("audio/mpeg"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mp3Bytes))),
			PTT:           proto.Bool(false),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil {
		_ = c.React("🎵")
	}
	return true, err
}
