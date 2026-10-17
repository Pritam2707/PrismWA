/**
 * PrismBot Dashboard Frontend Application
 * High-performance, lightweight Vanilla JS with luxury Obsidian & Cyber-Glass aesthetics
 */

let activityChartInstance = null;
let currentTab = "overview";
let allCachedChats = [];
let allTerminalLogs = [];

// ---------------------------------------------------------------------------
// 0. TOAST NOTIFICATION SYSTEM
// ---------------------------------------------------------------------------
function showToast(message, type = "info") {
  const container = document.getElementById("toast-container");
  if (!container) return;

  const toast = document.createElement("div");
  toast.className = `toast-item toast-${type}`;

  const iconName = type === "success" ? "check-circle" : type === "error" ? "alert-circle" : "info";

  toast.innerHTML = `
    <i data-lucide="${iconName}" class="w-4 h-4 flex-shrink-0"></i>
    <span class="text-xs font-medium leading-snug">${escapeHtml(message)}</span>
  `;

  container.appendChild(toast);
  if (window.lucide) {
    lucide.createIcons({ root: toast });
  }

  setTimeout(() => {
    toast.style.animation = "toast-out 0.25s ease-in forwards";
    setTimeout(() => toast.remove(), 250);
  }, 3500);
}

// Initialize on DOM ready
document.addEventListener("DOMContentLoaded", () => {
  if (window.lucide) lucide.createIcons();
  setupNavigation();
  setupEventListeners();
  initActivityChart();
  initSseLogStream();

  // Initial data load
  fetchStatus();
  fetchActiveTabData(currentTab);

  // Auto-refresh metrics every 4 seconds (ultra-low CPU footprint)
  setInterval(fetchStatus, 4000);
});

// ---------------------------------------------------------------------------
// 1. NAVIGATION & TAB SWITCHING
// ---------------------------------------------------------------------------
function setupNavigation() {
  const links = document.querySelectorAll(".nav-link");
  const panels = document.querySelectorAll(".tab-panel");
  const title = document.getElementById("currentTabTitle");
  const subTitle = document.getElementById("currentSubTitle");

  const tabTitles = {
    overview: { title: "Overview", sub: "System telemetry & live metrics" },
    controls: { title: "Chats & Contacts", sub: "Names, nicknames, and chat protections" },
    broadcast: { title: "Broadcast Studio", sub: "Send paced announcements across groups and DMs" },
    moderation: { title: "Moderation & Mutes", sub: "Manage muted users and anti-spam locks" },
    welcome: { title: "Welcomer Studio", sub: "Automated group greeting banners & templates" },
    personas: { title: "AI Personas & Tester", sub: "Character prompts & live interactive sandbox" },
    autoreply: { title: "Auto-Reply / Away", sub: "Away mode, trigger keywords, visitor inbox" },
    analytics: { title: "Chat Analytics", sub: "Member rankings and message breakdown" },
    audit: { title: "Audit Vault", sub: "Recovered deleted messages and view-once media" },
    storage: { title: "Storage Cleaner", sub: "Disk usage monitoring and cache cleaner" },
    notes: { title: "Notes Explorer", sub: "Virtual folder tree and document storage" },
    reminders: { title: "Reminders & Scheduler", sub: "Active countdown timers and reminders" },
    messenger: { title: "Direct Messenger", sub: "Send direct messages to contacts or groups" },
    commands: { title: "Command Cheat Sheet", sub: "50+ bot commands library & quick copy" },
    terminal: { title: "Live Activity Console", sub: "Real-time SSE event stream" },
  };

  function switchTab(tabId) {
    currentTab = tabId;
    links.forEach((l) => l.classList.toggle("active", l.dataset.tab === tabId));
    panels.forEach((p) => p.classList.toggle("hidden", p.id !== `tab-${tabId}`));

    if (tabTitles[tabId]) {
      if (title) title.textContent = tabTitles[tabId].title;
      if (subTitle) subTitle.textContent = tabTitles[tabId].sub;
    }

    fetchActiveTabData(tabId);
    if (window.lucide) lucide.createIcons();

    // Close mobile drawer on switch
    document.getElementById("sidebar")?.classList.add("-translate-x-full");
  }

  links.forEach((link) => {
    link.addEventListener("click", (e) => {
      e.preventDefault();
      const tabId = link.dataset.tab;
      window.location.hash = tabId;
      switchTab(tabId);
    });
  });

  // Handle Hash routing
  const initialHash = window.location.hash.replace("#", "");
  if (initialHash && tabTitles[initialHash]) {
    switchTab(initialHash);
  }
}

function setupEventListeners() {
  // Mobile sidebar toggles
  document.getElementById("mobileMenuBtn")?.addEventListener("click", () => {
    document.getElementById("sidebar")?.classList.toggle("-translate-x-full");
  });

  document.getElementById("closeSidebarBtn")?.addEventListener("click", () => {
    document.getElementById("sidebar")?.classList.add("-translate-x-full");
  });

  // Quick Broadcast Launcher in top bar
  document.getElementById("quickBroadcastLauncherBtn")?.addEventListener("click", () => {
    window.location.hash = "broadcast";
    const broadcastLink = document.querySelector('[data-tab="broadcast"]');
    broadcastLink?.click();
  });

  // Quick Sleep/Wake toggle
  document.getElementById("quickSleepBtn")?.addEventListener("click", async () => {
    try {
      const res = await fetch("/api/control/toggle-sleep", { method: "POST" });
      const data = await res.json();
      updateSleepUI(data.isSleeping);
      showToast(data.isSleeping ? "Bot entered Sleep Mode" : "Bot woke up & resumed full ops", "info");
    } catch (e) {
      console.error(e);
      showToast("Failed to toggle sleep mode", "error");
    }
  });

  // Manual Refresh
  document.getElementById("refreshBtn")?.addEventListener("click", () => {
    fetchStatus();
    fetchActiveTabData(currentTab);
    showToast("Dashboard refreshed", "info");
  });

  // Chat Directory Search & Filter
  document.getElementById("chatSearchInput")?.addEventListener("input", renderFilteredChats);
  document.getElementById("chatTypeFilter")?.addEventListener("change", renderFilteredChats);

  // Clear Terminal Logs
  document.getElementById("clearLogsBtn")?.addEventListener("click", async () => {
    try {
      await fetch("/api/logs", { method: "DELETE" });
      const output = document.getElementById("terminalOutput");
      if (output) output.innerHTML = "";
      allTerminalLogs = [];
      const badge = document.getElementById("logBadge");
      if (badge) badge.textContent = "0";
      showToast("Terminal logs cleared", "info");
    } catch (e) {
      showToast("Failed to clear logs", "error");
    }
  });

  document.getElementById("logTypeFilter")?.addEventListener("change", renderFilteredLogs);

  // Auto-Reply Save Form
  document.getElementById("saveAutoReplyBtn")?.addEventListener("click", async () => {
    const enabled = document.getElementById("autoReplyToggle").checked;
    const mode = document.getElementById("autoReplyModeSelect").value;
    const message = document.getElementById("autoReplyMessageInput").value;

    try {
      await fetch("/api/autoreply", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled, mode, message }),
      });

      showToast("Auto-Reply settings saved successfully!", "success");
      fetchAutoReplyData();
    } catch (e) {
      showToast("Failed to save Auto-Reply settings", "error");
    }
  });

  // Add Trigger
  document.getElementById("addTriggerBtn")?.addEventListener("click", async () => {
    const phrase = document.getElementById("triggerKeywordInput").value.trim();
    const reply = document.getElementById("triggerReplyInput").value.trim();
    if (!phrase || !reply) return showToast("Please enter both trigger keyword and reply text", "error");

    try {
      await fetch("/api/autoreply", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ addTrigger: { phrase, reply } }),
      });

      document.getElementById("triggerKeywordInput").value = "";
      document.getElementById("triggerReplyInput").value = "";
      showToast(`Keyword trigger "${phrase}" added`, "success");
      fetchAutoReplyData();
    } catch (e) {
      showToast("Failed to add trigger", "error");
    }
  });

  // Add Target JID
  document.getElementById("addTargetBtn")?.addEventListener("click", async () => {
    const target = document.getElementById("targetJidInput").value.trim();
    if (!target) return showToast("Please enter phone number or JID", "error");

    try {
      await fetch("/api/autoreply", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ addTarget: target }),
      });

      document.getElementById("targetJidInput").value = "";
      showToast("Target JID added to auto-reply list", "success");
      fetchAutoReplyData();
    } catch (e) {
      showToast("Failed to add target JID", "error");
    }
  });

  // Flush Notes
  document.getElementById("flushNotesBtn")?.addEventListener("click", async () => {
    if (!confirm("Flush all visitor messages?")) return;
    try {
      await fetch("/api/autoreply/flush", { method: "POST" });
      showToast("Visitor notes cleared", "info");
      fetchAutoReplyData();
    } catch (e) {
      showToast("Failed to flush notes", "error");
    }
  });

  // Broadcast Launch
  document.getElementById("sendBroadcastBtn")?.addEventListener("click", launchBroadcast);

  // Welcomer Save
  document.getElementById("saveWelcomeBtn")?.addEventListener("click", saveWelcomerConfig);

  // Live Persona Tester Sandbox
  document.getElementById("sendPersonaTestBtn")?.addEventListener("click", runPersonaLiveTest);
  document.getElementById("testPersonaPrompt")?.addEventListener("keydown", (e) => {
    if (e.key === "Enter") runPersonaLiveTest();
  });

  // Direct Message Send
  document.getElementById("sendDirectMsgBtn")?.addEventListener("click", sendDirectMessage);
  document.getElementById("directRecipientSelect")?.addEventListener("change", (e) => {
    if (e.target.value) {
      document.getElementById("directJidInput").value = e.target.value;
    }
  });

  // Storage Cleaner Buttons
  document.querySelectorAll(".clean-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const target = btn.dataset.target;
      if (!confirm(`Are you sure you want to clean: ${target}?`)) return;
      try {
        await fetch("/api/storage/clean", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ target }),
        });
        fetchStorageData();
        showToast(`Storage cleaned: ${target}`, "success");
      } catch (e) {
        showToast(`Failed to clean ${target}`, "error");
      }
    });
  });

  // Command Cheat Sheet search & filter
  document.getElementById("commandSearchInput")?.addEventListener("input", renderCommandCheatSheet);
  document.getElementById("commandCategoryFilter")?.addEventListener("change", renderCommandCheatSheet);

  // Modals event listeners
  setupModals();
}

// ---------------------------------------------------------------------------
// 2. STATUS & TELEMETRY
// ---------------------------------------------------------------------------
async function fetchStatus() {
  try {
    const res = await fetch("/api/status");
    const data = await res.json();

    if (data.status !== "ok") return;

    // Metrics
    const botStatus = document.getElementById("metricBotStatus");
    if (botStatus) botStatus.textContent = data.bot.connectionState === "open" ? "Online" : "Connecting";

    const totalMsgs = document.getElementById("metricTotalMsgs");
    if (totalMsgs) totalMsgs.textContent = (data.bot.totalCachedMessages || 0).toLocaleString();

    const activeChats = document.getElementById("metricActiveChats");
    if (activeChats) activeChats.textContent = (data.bot.knownContactsCount || 0).toLocaleString();

    const rss = document.getElementById("metricRss");
    if (rss) rss.textContent = `${data.system.rssMB || 0} MB`;

    const platform = document.getElementById("metricPlatform");
    if (platform) platform.textContent = `${data.system.platform} (${data.system.arch}) • Bot v2.0`;

    // Host Profile
    if (data.bot.user) {
      const profileName = document.getElementById("profileName");
      if (profileName) profileName.textContent = data.bot.user.name || "PrismBot";
      const profileJid = document.getElementById("profileJid");
      if (profileJid) profileJid.textContent = data.bot.user.phone || data.bot.user.id;
      const sidebarPhone = document.getElementById("sidebarBotPhone");
      if (sidebarPhone) sidebarPhone.textContent = data.bot.user.phone || "WhatsMeow • Go";
    }

    // Detail counts
    const sockState = document.getElementById("detailSockState");
    if (sockState) sockState.textContent = data.bot.connectionState === "open" ? "Connected" : data.bot.connectionState;

    const antiDeleteCount = document.getElementById("detailAntiDeleteCount");
    if (antiDeleteCount) antiDeleteCount.textContent = `${data.bot.antiDeleteChatsCount || 0} chats`;

    const remindersCount = document.getElementById("detailRemindersCount");
    if (remindersCount) remindersCount.textContent = `${data.bot.remindersCount || 0} scheduled`;

    const mutedCount = document.getElementById("detailMutedCount");
    if (mutedCount) mutedCount.textContent = `${data.bot.mutedCount || 0} users`;

    // RAM / Heap gauge
    const heapText = document.getElementById("heapText");
    if (heapText) heapText.textContent = `${data.system.heapUsedMB || 0} / ${data.system.totalMemMB || 1024} MB`;

    const heapPercent = Math.min(100, Math.round(((data.system.heapUsedMB || 0) / (data.system.totalMemMB || 1024)) * 100));
    const heapBar = document.getElementById("heapBar");
    if (heapBar) heapBar.style.width = `${heapPercent}%`;

    // Uptime
    const uptimeMins = Math.floor((data.bot.uptimeSecs || 0) / 60);
    const uptimeHours = Math.floor(uptimeMins / 60);
    const formattedUptime = uptimeHours > 0 ? `${uptimeHours}h ${uptimeMins % 60}m` : `${uptimeMins}m`;

    const uptimeBadge = document.getElementById("uptimeBadge");
    if (uptimeBadge) uptimeBadge.textContent = formattedUptime;

    const uptimeSub = document.getElementById("metricUptimeSub");
    if (uptimeSub) uptimeSub.textContent = `Uptime: ${formattedUptime}`;

    updateSleepUI(data.bot.isSleeping);
  } catch (e) {
    const dot = document.getElementById("statusDot");
    if (dot) dot.classList.replace("bg-emerald-400", "bg-red-500");
    const st = document.getElementById("statusText");
    if (st) st.textContent = "Offline / Reconnecting";
  }
}

function updateSleepUI(isSleeping) {
  const label = document.getElementById("quickSleepLabel");
  const badge = document.getElementById("metricSleepState");
  if (isSleeping) {
    if (label) label.textContent = "Wake Up";
    if (badge) {
      badge.textContent = "Sleeping";
      badge.className = "text-xs text-indigo-400 font-medium";
    }
  } else {
    if (label) label.textContent = "Sleep";
    if (badge) {
      badge.textContent = "Awake";
      badge.className = "text-xs text-emerald-400 font-medium";
    }
  }
}

// ---------------------------------------------------------------------------
// 3. TAB DATA ROUTER
// ---------------------------------------------------------------------------
function fetchActiveTabData(tabId) {
  switch (tabId) {
    case "overview":
      fetchStatsData();
      break;
    case "controls":
      fetchChatsData();
      break;
    case "broadcast":
      break;
    case "moderation":
      fetchMutesData();
      break;
    case "welcome":
      fetchWelcomeData();
      break;
    case "personas":
      fetchPersonasData();
      break;
    case "autoreply":
      fetchAutoReplyData();
      break;
    case "analytics":
      fetchAnalyticsData();
      break;
    case "audit":
      fetchAuditData();
      break;
    case "storage":
      fetchStorageData();
      break;
    case "notes":
      fetchNotesData();
      break;
    case "reminders":
      fetchRemindersData();
      break;
    case "messenger":
      populateMessengerRecipients();
      break;
    case "commands":
      renderCommandCheatSheet();
      break;
    case "terminal":
      renderFilteredLogs();
      break;
  }
}

// ---------------------------------------------------------------------------
// 4. ACTIVITY CHART
// ---------------------------------------------------------------------------
function initActivityChart() {
  const ctx = document.getElementById("activityChart")?.getContext("2d");
  if (!ctx) return;

  activityChartInstance = new Chart(ctx, {
    type: "doughnut",
    data: {
      labels: ["Text Messages", "Photos & Videos", "Voice Audio", "Stickers"],
      datasets: [
        {
          data: [1, 0, 0, 0],
          backgroundColor: ["#10b981", "#6366f1", "#f59e0b", "#ec4899"],
          borderWidth: 0,
          hoverOffset: 6,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          position: "right",
          labels: { color: "#94a3b8", font: { family: "Inter", size: 12 } },
        },
      },
      cutout: "70%",
    },
  });
}

async function fetchStatsData() {
  try {
    const res = await fetch("/api/stats");
    const stats = await res.json();

    let text = 0, media = 0, audio = 0, stickers = 0, total = 0;

    for (const chat of Object.values(stats)) {
      total += chat.totalMessages || 0;
      if (chat.breakdown) {
        text += chat.breakdown.text || 0;
        media += chat.breakdown.media || 0;
        audio += chat.breakdown.audio || 0;
        stickers += chat.breakdown.stickers || 0;
      }
    }

    const totalEl = document.getElementById("chartTotalCount");
    if (totalEl) totalEl.textContent = `Total: ${total.toLocaleString()} messages`;

    if (activityChartInstance) {
      activityChartInstance.data.datasets[0].data = [text || 1, media, audio, stickers];
      activityChartInstance.update();
    }
  } catch (e) {}
}

// ---------------------------------------------------------------------------
// 5. CHAT DIRECTORY & NICKNAMES
// ---------------------------------------------------------------------------
async function fetchChatsData() {
  try {
    const res = await fetch("/api/chats");
    allCachedChats = await res.json();
    renderFilteredChats();
  } catch (e) {
    console.error(e);
  }
}

function renderFilteredChats() {
  const query = (document.getElementById("chatSearchInput")?.value || "").toLowerCase().trim();
  const filterType = document.getElementById("chatTypeFilter")?.value || "all";
  const tbody = document.getElementById("chatsTableBody");
  if (!tbody) return;

  let filtered = allCachedChats.filter((c) => {
    if (filterType === "groups" && !c.isGroup) return false;
    if (filterType === "dms" && c.isGroup) return false;
    if (query) {
      const matchName = (c.name || "").toLowerCase().includes(query);
      const matchPhone = (c.phone || "").toLowerCase().includes(query);
      const matchJid = (c.jid || "").toLowerCase().includes(query);
      return matchName || matchPhone || matchJid;
    }
    return true;
  });

  if (filtered.length === 0) {
    tbody.innerHTML = `<tr><td colspan="6" class="p-6 text-center text-slate-500">No matching contacts or groups found.</td></tr>`;
    return;
  }

  tbody.innerHTML = filtered
    .map((c) => {
      const badge = c.isGroup
        ? `<span class="px-1.5 py-0.5 rounded bg-indigo-500/10 text-indigo-400 font-mono text-[10px] border border-indigo-500/20">GROUP</span>`
        : `<span class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 font-mono text-[10px] border border-emerald-500/20">DM</span>`;

      return `
      <tr class="hover:bg-dark-700/20 transition">
        <td class="p-4">
          <div class="font-bold text-white flex items-center gap-2">
            ${badge}
            <span class="truncate max-w-[180px] sm:max-w-xs">${escapeHtml(c.name)}</span>
            <button onclick="openRenameModal('${c.jid}', '${escapeHtml(c.name).replace(/'/g, "\\'")}')" class="text-slate-400 hover:text-emerald-400 transition" title="Edit Name / Nickname">
              <i data-lucide="edit-3" class="w-3.5 h-3.5"></i>
            </button>
          </div>
        </td>
        <td class="p-4 font-mono text-slate-400 text-[11px]">${c.phone || c.jid}</td>
        <td class="p-4 font-mono text-slate-200">${(c.totalMessages || 0).toLocaleString()}</td>
        <td class="p-4">
          <button onclick="toggleAntiDelete('${c.jid}', ${!c.antiDelete})" class="px-2.5 py-1 rounded-lg text-xs font-semibold transition ${
            c.antiDelete ? "bg-emerald-500/20 text-emerald-400 border border-emerald-500/30" : "bg-dark-700 text-slate-400"
          }">
            ${c.antiDelete ? "Enabled" : "Disabled"}
          </button>
        </td>
        <td class="p-4">
          <button onclick="toggleFeatures('${c.jid}', ${!c.features})" class="px-2.5 py-1 rounded-lg text-xs font-semibold transition ${
            c.features ? "bg-indigo-500/20 text-indigo-400 border border-indigo-500/30" : "bg-dark-700 text-slate-400"
          }">
            ${c.features ? "Active" : "Off"}
          </button>
        </td>
        <td class="p-4 text-right">
          <button onclick="quickMessageChat('${c.jid}')" class="p-2 rounded-lg bg-dark-700 hover:bg-dark-600 text-slate-300 transition" title="Send WhatsApp Message">
            <i data-lucide="send" class="w-3.5 h-3.5 text-emerald-400"></i>
          </button>
        </td>
      </tr>`;
    })
    .join("");

  if (window.lucide) lucide.createIcons({ root: tbody });
}

window.toggleAntiDelete = async (jid, enable) => {
  try {
    await fetch("/api/control/toggle-antidelete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jid, enable }),
    });
    showToast(`Anti-Delete ${enable ? "enabled" : "disabled"} for chat`, "info");
    fetchChatsData();
  } catch (e) {
    showToast("Failed to toggle anti-delete", "error");
  }
};

window.toggleFeatures = async (jid, enable) => {
  try {
    await fetch("/api/control/toggle-features", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jid, enable }),
    });
    showToast(`Interactive features ${enable ? "activated" : "disabled"} for chat`, "info");
    fetchChatsData();
  } catch (e) {
    showToast("Failed to toggle features", "error");
  }
};

window.quickMessageChat = (jid) => {
  window.location.hash = "messenger";
  document.querySelector('[data-tab="messenger"]')?.click();
  const jidInput = document.getElementById("directJidInput");
  if (jidInput) jidInput.value = jid;
  document.getElementById("directMsgInput")?.focus();
};

// ---------------------------------------------------------------------------
// 6. BROADCAST ANNOUNCEMENT STUDIO
// ---------------------------------------------------------------------------
async function launchBroadcast() {
  const text = document.getElementById("broadcastTextInput").value.trim();
  if (!text) return showToast("Please write your announcement message first", "error");

  const targetType = document.querySelector('input[name="broadcastTarget"]:checked')?.value || "groups";

  if (!confirm(`Launch announcement broadcast to: ${targetType.toUpperCase()}?`)) return;

  const btn = document.getElementById("sendBroadcastBtn");
  const progressBox = document.getElementById("broadcastProgressBox");
  const progressBar = document.getElementById("broadcastProgressBar");
  const progressText = document.getElementById("broadcastProgressText");

  if (btn) btn.disabled = true;
  if (progressBox) progressBox.classList.remove("hidden");
  if (progressBar) progressBar.style.width = "40%";
  if (progressText) progressText.textContent = "Broadcasting messages...";

  try {
    const res = await fetch("/api/message/broadcast", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ text, targetType }),
    });
    const result = await res.json();

    if (progressBar) progressBar.style.width = "100%";
    if (progressText) progressText.textContent = "Completed!";
    showToast(`Broadcast complete! Sent to ${result.sent} chats (${result.failed} failed).`, "success");
    document.getElementById("broadcastTextInput").value = "";
  } catch (err) {
    showToast(`Broadcast error: ${err.message}`, "error");
  } finally {
    if (btn) btn.disabled = false;
    setTimeout(() => {
      if (progressBox) progressBox.classList.add("hidden");
    }, 3000);
  }
}

// ---------------------------------------------------------------------------
// 7. MODERATION & MUTED USERS
// ---------------------------------------------------------------------------
async function fetchMutesData() {
  try {
    const res = await fetch("/api/moderation/mutes");
    const mutes = await res.json();
    const tbody = document.getElementById("mutedUsersTableBody");
    if (!tbody) return;

    if (!Array.isArray(mutes) || mutes.length === 0) {
      tbody.innerHTML = `<tr><td colspan="4" class="p-6 text-center text-slate-500">No users are currently muted.</td></tr>`;
      return;
    }

    tbody.innerHTML = mutes
      .map(
        (m) => `
      <tr class="hover:bg-dark-700/20 transition">
        <td class="p-4 font-bold text-white">${escapeHtml(m.senderName || m.senderId)}</td>
        <td class="p-4 font-mono text-slate-400 text-[11px]">${m.phone || m.senderId}</td>
        <td class="p-4 text-slate-300 font-medium">${escapeHtml(m.chatName || m.jid)}</td>
        <td class="p-4 text-right">
          <button onclick="unmuteUser('${m.jid}', '${m.senderId}')" class="px-3 py-1 rounded-lg bg-red-500/20 hover:bg-red-500/30 text-red-400 border border-red-500/30 text-xs font-semibold transition">
            Unmute
          </button>
        </td>
      </tr>`
      )
      .join("");
  } catch (e) {
    console.error(e);
  }
}

window.unmuteUser = async (jid, senderId) => {
  if (!confirm(`Unmute this user in ${jid}?`)) return;
  try {
    await fetch(`/api/moderation/mute/${encodeURIComponent(jid)}/${encodeURIComponent(senderId)}`, { method: "DELETE" });
    showToast("User unmuted successfully", "success");
    fetchMutesData();
  } catch (e) {
    showToast("Failed to unmute user", "error");
  }
};

// ---------------------------------------------------------------------------
// 8. GROUP WELCOMER STUDIO
// ---------------------------------------------------------------------------
async function fetchWelcomeData() {
  try {
    const [welcomeRes, chatsRes] = await Promise.all([fetch("/api/welcome"), fetch("/api/chats")]);
    const welcomeData = await welcomeRes.json();
    const chats = await chatsRes.json();

    // Populate Group select dropdown
    const groupSelect = document.getElementById("welcomeGroupSelect");
    const groupChats = chats.filter((c) => c.isGroup);

    if (groupSelect) {
      groupSelect.innerHTML = `<option value="">Select a group...</option>` +
        groupChats.map((g) => `<option value="${g.jid}">${escapeHtml(g.name)} (${g.phone || g.jid})</option>`).join("");

      groupSelect.onchange = (e) => {
        const selectedJid = e.target.value;
        if (welcomeData[selectedJid]) {
          document.getElementById("welcomeToggle").checked = !!welcomeData[selectedJid].enabled;
          document.getElementById("welcomeTextInput").value = welcomeData[selectedJid].text || "";
        } else {
          document.getElementById("welcomeToggle").checked = true;
          document.getElementById("welcomeTextInput").value = "👋 Welcome @user to {group}!";
        }
      };
    }

    // Render cards
    const container = document.getElementById("welcomeCardsContainer");
    if (!container) return;
    const entries = Object.entries(welcomeData || {});

    if (entries.length === 0) {
      container.innerHTML = `<p class="text-slate-500 col-span-full text-center py-6">No custom group welcome messages configured yet.</p>`;
      return;
    }

    container.innerHTML = entries
      .map(
        ([jid, cfg]) => `
      <div class="p-4 rounded-xl bg-dark-700/40 border border-slate-700/60 space-y-2">
        <div class="flex items-center justify-between">
          <h5 class="font-bold text-white text-xs truncate max-w-[200px]">${escapeHtml(cfg.groupName || jid)}</h5>
          <span class="px-2 py-0.5 rounded text-[10px] font-mono ${cfg.enabled ? "bg-teal-500/20 text-teal-400" : "bg-dark-700 text-slate-400"}">${cfg.enabled ? "Active" : "Disabled"}</span>
        </div>
        <p class="text-slate-300 text-xs line-clamp-2 bg-dark-800/60 p-2 rounded-lg font-sans">${escapeHtml(cfg.text)}</p>
      </div>`
      )
      .join("");
  } catch (e) {
    console.error(e);
  }
}

async function saveWelcomerConfig() {
  const jid = document.getElementById("welcomeGroupSelect").value;
  const enabled = document.getElementById("welcomeToggle").checked;
  const text = document.getElementById("welcomeTextInput").value.trim();

  if (!jid) return showToast("Please select a group chat first", "error");
  if (!text) return showToast("Please enter welcome text", "error");

  try {
    await fetch("/api/welcome", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jid, enabled, text }),
    });

    showToast("Welcome configuration saved successfully!", "success");
    fetchWelcomeData();
  } catch (e) {
    showToast("Failed to save welcome configuration", "error");
  }
}

// ---------------------------------------------------------------------------
// 9. AI PERSONAS & LIVE TEST PLAYGROUND
// ---------------------------------------------------------------------------
async function fetchPersonasData() {
  try {
    const res = await fetch("/api/personas");
    const data = await res.json();
    const grid = document.getElementById("personasListGrid");
    if (!grid) return;
    const personas = Object.entries(data.personas || {});

    if (personas.length === 0) {
      grid.innerHTML = `<div class="p-8 text-center text-slate-500 col-span-full">No personas found.</div>`;
      return;
    }

    grid.innerHTML = personas
      .map(([key, p]) => {
        const label = p.label || p.Name || p.name || key;
        const prefix = p.prefix || p.Prefix || ("!" + key);
        const flavor = p.flavor || p.Prompt || p.prompt || "";
        const stickersCount = Object.keys(p.stickers || {}).length;

        return `
        <div class="p-5 rounded-2xl glass-card-interactive space-y-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2.5">
              <div class="w-9 h-9 rounded-xl bg-gradient-to-br from-indigo-500 via-purple-500 to-pink-500 flex items-center justify-center font-bold text-white text-sm shadow-md">
                ${label.charAt(0).toUpperCase()}
              </div>
              <div>
                <h4 class="font-bold text-white text-sm">${escapeHtml(label)}</h4>
                <span class="text-[11px] font-mono text-emerald-400 font-semibold">${escapeHtml(prefix)}</span>
              </div>
            </div>
            <span class="text-[10px] px-2 py-0.5 rounded bg-dark-700 text-slate-400 font-mono">${stickersCount} stickers</span>
          </div>
          <p class="text-xs text-slate-400 line-clamp-3 leading-relaxed">${escapeHtml(flavor)}</p>
        </div>`;
      })
      .join("");

    // Populate testPersonaSelect dropdown dynamically
    const select = document.getElementById("testPersonaSelect");
    if (select) {
      const currentVal = select.value;
      select.innerHTML = personas
        .map(([key, p]) => {
          const label = p.label || p.Name || p.name || key;
          const prefix = p.prefix || p.Prefix || ("!" + key);
          return `<option value="${key}">${escapeHtml(label)} (${escapeHtml(prefix)})</option>`;
        })
        .join("");
      if (currentVal && select.querySelector(`option[value="${currentVal}"]`)) {
        select.value = currentVal;
      }
    }
  } catch (e) {
    console.error(e);
  }
}

async function runPersonaLiveTest() {
  const select = document.getElementById("testPersonaSelect");
  const personaKey = select ? select.value : "zero2";
  const promptInput = document.getElementById("testPersonaPrompt");
  const prompt = promptInput ? promptInput.value.trim() : "";
  if (!prompt) return showToast("Please type a message to test", "error");

  const btn = document.getElementById("sendPersonaTestBtn");
  const outputBox = document.getElementById("testPersonaOutputBox");
  const answerEl = document.getElementById("testOutputAnswer");
  const moodBadge = document.getElementById("testOutputMoodBadge");
  const nameEl = document.getElementById("testOutputPersonaName");

  if (btn) btn.disabled = true;
  if (outputBox) outputBox.classList.remove("hidden");
  if (answerEl) answerEl.textContent = "Generating response with Gemini 2.5 Flash...";

  try {
    const res = await fetch("/api/personas/test", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ personaKey, prompt }),
    });
    const result = await res.json();

    if (result.success === false) {
      if (answerEl) answerEl.textContent = `Error: ${result.error || "Unknown AI error"}`;
      showToast(`AI generation error: ${result.error || "Unknown error"}`, "error");
      return;
    }

    if (nameEl && select && select.selectedOptions[0]) {
      nameEl.textContent = select.selectedOptions[0].text;
    }
    if (moodBadge) {
      moodBadge.textContent = result.mood ? `[MOOD: ${result.mood}]` : "[MOOD: standard]";
    }
    if (answerEl) {
      answerEl.textContent = result.answer || "No response generated.";
    }
    showToast("AI response generated", "success");
  } catch (err) {
    if (answerEl) answerEl.textContent = `Error: ${err.message}`;
    showToast(`AI generation error: ${err.message}`, "error");
  } finally {
    if (btn) btn.disabled = false;
  }
}

// ---------------------------------------------------------------------------
// 10. CHAT ANALYTICS & LEADERBOARDS
// ---------------------------------------------------------------------------
async function fetchAnalyticsData() {
  try {
    const res = await fetch("/api/stats");
    const stats = await res.json();
    const container = document.getElementById("analyticsContainer");
    if (!container) return;

    const chats = Object.entries(stats || {});
    if (chats.length === 0) {
      container.innerHTML = `<div class="p-8 text-center text-slate-500 col-span-full">No chat activity statistics recorded yet.</div>`;
      return;
    }

    container.innerHTML = chats
      .map(([jid, chat]) => {
        const users = Object.entries(chat.users || {})
          .sort(([, a], [, b]) => b.count - a.count)
          .slice(0, 5);

        return `
        <div class="p-6 rounded-2xl glass-card-interactive space-y-4">
          <div class="flex items-center justify-between border-b border-slate-800 pb-3">
            <div>
              <h4 class="font-bold text-white text-sm">${escapeHtml(chat.chatName || jid)}</h4>
              <p class="text-xs text-slate-500 font-mono">${chat.phone || jid} • ${(chat.totalMessages || 0).toLocaleString()} total msgs</p>
            </div>
            <button onclick="resetChatStats('${jid}')" class="px-2.5 py-1 rounded-xl bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 text-xs transition">
              Reset
            </button>
          </div>

          <div class="space-y-2 text-xs">
            <span class="font-semibold text-slate-400 uppercase tracking-wider text-[10px]">Top Active Members</span>
            ${users
              .map(([uid, u], idx) => {
                const pct = chat.totalMessages > 0 ? ((u.count / chat.totalMessages) * 100).toFixed(1) : 0;
                return `
                <div class="flex items-center justify-between py-1.5 border-b border-slate-800/40">
                  <span class="text-slate-200 font-medium">#${idx + 1} • ${escapeHtml(u.name || uid)} <span class="text-[11px] text-slate-500 font-mono">(${u.phone || uid})</span></span>
                  <span class="font-mono text-emerald-400 font-semibold">${u.count.toLocaleString()} (${pct}%)</span>
                </div>`;
              })
              .join("")}
          </div>
        </div>`;
      })
      .join("");
  } catch (e) {
    console.error(e);
  }
}

window.resetChatStats = async (jid) => {
  if (!confirm(`Reset stats for ${jid}?`)) return;
  try {
    await fetch(`/api/stats/${encodeURIComponent(jid)}`, { method: "DELETE" });
    showToast("Chat statistics reset", "info");
    fetchAnalyticsData();
  } catch (e) {
    showToast("Failed to reset chat stats", "error");
  }
};

// ---------------------------------------------------------------------------
// 11. AUDIT VAULT (RECOVERED DELETED MESSAGES & VIEW-ONCE)
// ---------------------------------------------------------------------------
let auditDataCache = null;

function renderDeletedMessages() {
  const delContainer = document.getElementById("deletedMsgsList");
  if (!delContainer || !auditDataCache) return;

  const grouped = auditDataCache.deletedMsgs || {};
  const selectedGroup = document.getElementById("deletedGroupFilter")?.value || "all";
  const searchQuery = document.getElementById("deletedSearchInput")?.value?.toLowerCase()?.trim() || "";

  let totalItems = 0;
  const groupsToRender = [];

  for (const [jid, rawItems] of Object.entries(grouped)) {
    if (!Array.isArray(rawItems) || rawItems.length === 0) continue;
    if (selectedGroup !== "all" && jid !== selectedGroup) continue;

    const chatName = rawItems[0]?.chatName || jid.split("@")[0] || "Chat";
    const isGroupChat = jid.endsWith("@g.us");

    // Filter items by search query
    let filteredItems = rawItems;
    if (searchQuery) {
      filteredItems = rawItems.filter((item) => {
        const text = (item.text || item.message?.conversation || "").toLowerCase();
        const sender = (item.senderName || item.pushName || item.sender || "").toLowerCase();
        return text.includes(searchQuery) || sender.includes(searchQuery);
      });
    }

    if (filteredItems.length > 0) {
      totalItems += filteredItems.length;
      groupsToRender.push({
        jid,
        chatName,
        isGroupChat,
        items: [...filteredItems].reverse(),
      });
    }
  }

  // Update count badge
  const delCountBadge = document.getElementById("deletedMsgsCountBadge");
  if (delCountBadge) delCountBadge.textContent = `${totalItems} items`;

  if (groupsToRender.length === 0) {
    delContainer.innerHTML = `<p class="text-xs text-slate-500 text-center py-6">No deleted messages match your criteria.</p>`;
    return;
  }

  delContainer.innerHTML = groupsToRender
    .map((grp) => {
      const groupIcon = grp.isGroupChat ? "users" : "message-square";
      const groupBadge = grp.isGroupChat ? "Group" : "DM";
      const jidShort = grp.jid.length > 22 ? grp.jid.slice(0, 10) + "..." + grp.jid.slice(-10) : grp.jid;

      const itemsHtml = grp.items
        .map((item) => {
          const time = item.deletedAt ? new Date(item.deletedAt).toLocaleTimeString() : "";
          let mediaPreview = "";
          if (item.localFilePath) {
            const fileName = item.localFilePath.split(/[\\/]/).pop();
            if (fileName.endsWith(".jpg") || fileName.endsWith(".webp")) {
              mediaPreview = `<img src="/data/media/${fileName}" class="w-24 h-24 object-cover rounded-xl border border-slate-700 mt-2 hover:scale-105 transition cursor-pointer">`;
            }
            if (fileName.endsWith(".mp3")) {
              mediaPreview = `<audio controls src="/data/media/${fileName}" class="w-full mt-2 h-8"></audio>`;
            }
          }

          return `
          <div class="p-3 rounded-xl bg-dark-800/70 border border-slate-700/40 text-xs space-y-1.5 hover:border-slate-600/60 transition">
            <div class="flex justify-between items-center font-semibold text-slate-300">
              <span class="text-emerald-400 flex items-center gap-1.5">
                <i data-lucide="user" class="w-3.5 h-3.5 text-emerald-500/70"></i>
                ${escapeHtml(item.senderName || item.pushName || item.sender || "User")}
                ${item.sender ? `<span class="text-slate-500 font-normal font-mono text-[10px]">(${escapeHtml(item.sender.split("@")[0])})</span>` : ""}
              </span>
              <div class="flex items-center gap-2">
                <span class="text-slate-500 font-mono text-[11px]">${time}</span>
                ${item.id ? `<button onclick="deleteAuditItem('${item.id}')" class="text-slate-500 hover:text-red-400 transition p-1" title="Delete record"><i data-lucide="trash-2" class="w-3.5 h-3.5"></i></button>` : ""}
              </div>
            </div>
            <p class="text-slate-200 leading-relaxed break-words">${escapeHtml(item.text || item.message?.conversation || item.message?.extendedTextMessage?.text || "[Media Payload]")}</p>
            ${mediaPreview}
          </div>`;
        })
        .join("");

      return `
      <div class="rounded-2xl bg-dark-700/30 border border-slate-700/50 p-3.5 space-y-3">
        <div class="flex items-center justify-between border-b border-slate-700/40 pb-2.5">
          <div class="flex items-center gap-2 min-w-0">
            <div class="p-1.5 rounded-lg bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
              <i data-lucide="${groupIcon}" class="w-4 h-4"></i>
            </div>
            <div class="min-w-0">
              <h5 class="font-bold text-white text-xs truncate">${escapeHtml(grp.chatName)}</h5>
              <p class="text-[10px] text-slate-500 font-mono truncate">${escapeHtml(jidShort)}</p>
            </div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <span class="text-[10px] px-2 py-0.5 rounded-full bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 font-mono font-semibold">${grp.items.length} msgs</span>
            <button onclick="deleteAuditChat('${grp.jid}')" class="text-slate-500 hover:text-red-400 transition p-1" title="Clear all deleted logs for this group">
              <i data-lucide="folder-x" class="w-3.5 h-3.5"></i>
            </button>
          </div>
        </div>
        <div class="space-y-2.5">
          ${itemsHtml}
        </div>
      </div>`;
    })
    .join("");

  if (window.lucide) lucide.createIcons();
}

async function fetchAuditData() {
  try {
    const res = await fetch("/api/audit/deleted");
    const data = await res.json();
    auditDataCache = data;

    const voContainer = document.getElementById("viewOnceList");
    const allVo = [];

    // Calculate groups and total items
    const grouped = data.deletedMsgs || {};
    const groupSelect = document.getElementById("deletedGroupFilter");
    const currentSelected = groupSelect ? groupSelect.value : "all";

    let totalDeletedCount = 0;
    const groupOptions = [];

    for (const [jid, items] of Object.entries(grouped)) {
      if (Array.isArray(items) && items.length > 0) {
        totalDeletedCount += items.length;
        const chatName = items[0]?.chatName || jid.split("@")[0] || "Chat";
        groupOptions.push({ jid, chatName, count: items.length });
      }
    }

    // Update group selector options
    if (groupSelect) {
      groupSelect.innerHTML = `<option value="all">📁 All Groups & Chats (${totalDeletedCount})</option>` +
        groupOptions
          .map((g) => `<option value="${escapeHtml(g.jid)}">👥 ${escapeHtml(g.chatName)} (${g.count})</option>`)
          .join("");
      if (currentSelected && (currentSelected === "all" || groupOptions.some((g) => g.jid === currentSelected))) {
        groupSelect.value = currentSelected;
      }
    }

    const delGroupsCountBadge = document.getElementById("deletedGroupsCountBadge");
    if (delGroupsCountBadge) delGroupsCountBadge.textContent = `${groupOptions.length} groups`;

    // Render grouped deleted messages
    renderDeletedMessages();

    // View-Once list
    for (const [jid, items] of Object.entries(data.deletedVo || {})) {
      if (Array.isArray(items)) {
        items.forEach((item) => allVo.push({ ...item, jid }));
      }
    }

    const voCountBadge = document.getElementById("viewOnceCountBadge");
    if (voCountBadge) voCountBadge.textContent = `${allVo.length} items`;

    if (voContainer) {
      if (allVo.length === 0) {
        voContainer.innerHTML = `<p class="text-xs text-slate-500 text-center py-6">No View-Once media intercepted.</p>`;
      } else {
        voContainer.innerHTML = allVo
          .reverse()
          .slice(0, 30)
          .map((item) => {
            const time = item.deletedAt ? new Date(item.deletedAt).toLocaleTimeString() : "";
            let mediaPreview = "";
            if (item.localFilePath) {
              const fileName = item.localFilePath.split(/[\\/]/).pop();
              if (fileName.endsWith(".jpg") || fileName.endsWith(".webp")) {
                mediaPreview = `<img src="/data/media/${fileName}" class="w-24 h-24 object-cover rounded-xl border border-slate-700 mt-2 hover:scale-105 transition cursor-pointer">`;
              }
              if (fileName.endsWith(".mp3")) {
                mediaPreview = `<audio controls src="/data/media/${fileName}" class="w-full mt-2 h-8"></audio>`;
              }
            }

            return `
            <div class="p-3.5 rounded-xl bg-dark-700/40 border border-slate-700/50 text-xs space-y-1.5">
              <div class="flex justify-between items-center font-semibold text-slate-300">
                <span class="text-indigo-400">👤 ${escapeHtml(item.senderName || item.sender || "User")} <span class="text-slate-500 font-normal font-mono">in ${escapeHtml(item.chatName || item.jid || "Chat")}</span></span>
                <span class="text-slate-500 font-mono text-[11px]">${time}</span>
              </div>
              ${mediaPreview}
            </div>`;
          })
          .join("");
      }
    }

    // Attach filter listeners once
    if (groupSelect && !groupSelect.dataset.hasListener) {
      groupSelect.dataset.hasListener = "true";
      groupSelect.addEventListener("change", renderDeletedMessages);
    }
    const searchInput = document.getElementById("deletedSearchInput");
    if (searchInput && !searchInput.dataset.hasListener) {
      searchInput.dataset.hasListener = "true";
      searchInput.addEventListener("input", renderDeletedMessages);
    }

    if (window.lucide) lucide.createIcons();
  } catch (e) {
    console.error(e);
  }
}

window.deleteAuditItem = async (id) => {
  if (!confirm("Remove this entry from the audit vault?")) return;
  try {
    const res = await fetch(`/api/audit/deleted/${encodeURIComponent(id)}`, { method: "DELETE" });
    if (res.ok) {
      showToast("Audit item removed", "info");
      fetchAuditData();
    } else {
      showToast("Failed to remove item", "error");
    }
  } catch (e) {
    showToast("Error deleting item", "error");
  }
};

window.deleteAuditChat = async (chatJID) => {
  if (!confirm(`Clear all deleted message records for this group?`)) return;
  try {
    const res = await fetch(`/api/audit/deleted/chat/${encodeURIComponent(chatJID)}`, { method: "DELETE" });
    if (res.ok) {
      showToast("Group deleted audit records cleared", "info");
      fetchAuditData();
    } else {
      showToast("Failed to clear group records", "error");
    }
  } catch (e) {
    showToast("Error clearing group records", "error");
  }
};

// ---------------------------------------------------------------------------
// 12. STORAGE CLEANER & NOTES
// ---------------------------------------------------------------------------
async function fetchStorageData() {
  try {
    const res = await fetch("/api/storage");
    const data = await res.json();

    const media = document.getElementById("storageMedia");
    if (media) media.textContent = data.media?.formatted || "0 B";

    const stickers = document.getElementById("storageStickers");
    if (stickers) stickers.textContent = data.stickers?.formatted || "0 B";

    const deleted = document.getElementById("storageDeleted");
    if (deleted) deleted.textContent = data.deletedMsgs?.formatted || "0 B";

    const total = document.getElementById("storageTotal");
    if (total) total.textContent = data.total?.formatted || "0 B";
  } catch (e) {
    console.error(e);
  }
}

async function fetchNotesData() {
  try {
    const res = await fetch("/api/notes");
    const notes = await res.json();
    const container = document.getElementById("notesContainer");
    if (!container) return;
    const keys = Object.keys(notes || {});

    if (keys.length === 0) {
      container.innerHTML = `<p class="text-slate-500 py-6 text-center font-sans">No virtual notes stored yet.</p>`;
      return;
    }

    container.innerHTML = keys
      .map((key) => {
        const item = notes[key];
        const typeBadge = item.mediaType && item.mediaType !== "TXT"
          ? `<span class="px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 font-mono text-[10px] border border-indigo-500/30 font-semibold">${escapeHtml(item.mediaType)}</span>`
          : `<span class="px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300 font-mono text-[10px] border border-emerald-500/30 font-semibold">NOTE</span>`;

        const displayContent = item.content || item.fileName || (item.hasMedia ? "[Media File]" : "(empty)");

        return `
        <div class="p-3.5 rounded-xl bg-dark-700/30 border border-slate-700/40 flex items-center justify-between font-mono gap-3 hover:border-slate-600 transition">
          <div class="space-y-1 min-w-0 flex-1">
            <div class="flex items-center gap-2">
              ${typeBadge}
              <span class="text-emerald-400 font-semibold truncate text-xs">${escapeHtml(key)}</span>
            </div>
            <p class="text-[12px] text-slate-300 font-sans truncate max-w-2xl">${escapeHtml(displayContent)}</p>
          </div>
          <button onclick="deleteNote('${key.replace(/'/g, "\\'")}')" class="text-slate-500 hover:text-red-400 transition p-1.5 rounded-lg hover:bg-red-500/10 flex-shrink-0" title="Delete note">
            <i data-lucide="trash" class="w-3.5 h-3.5"></i>
          </button>
        </div>`;
      })
      .join("");

    if (window.lucide) lucide.createIcons({ root: container });
  } catch (e) {
    console.error(e);
  }
}

window.deleteNote = async (path) => {
  if (!confirm(`Delete note: ${path}?`)) return;
  try {
    await fetch("/api/notes", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path }),
    });
    showToast(`Deleted note: ${path}`, "info");
    fetchNotesData();
  } catch (e) {
    showToast("Failed to delete note", "error");
  }
};

// ---------------------------------------------------------------------------
// 13. REMINDERS & SCHEDULER
// ---------------------------------------------------------------------------
async function fetchRemindersData() {
  try {
    const res = await fetch("/api/reminders");
    const reminders = await res.json();
    const tbody = document.getElementById("remindersTableBody");
    if (!tbody) return;

    if (!Array.isArray(reminders) || reminders.length === 0) {
      tbody.innerHTML = `<tr><td colspan="4" class="p-6 text-center text-slate-500">No reminders currently scheduled.</td></tr>`;
      return;
    }

    const now = Date.now();
    tbody.innerHTML = reminders
      .map((r) => {
        const diffMins = Math.max(0, Math.round((r.targetTime - now) / 60000));
        return `
        <tr class="hover:bg-dark-700/20 transition">
          <td class="p-4 font-medium text-white">${escapeHtml(r.text)}</td>
          <td class="p-4 text-slate-300">${escapeHtml(r.chatName || r.jid)}</td>
          <td class="p-4 font-mono text-emerald-400 font-semibold">${diffMins} min(s)</td>
          <td class="p-4 text-right">
            <button onclick="deleteReminder('${r.id}')" class="p-1.5 rounded-lg bg-red-500/20 hover:bg-red-500/30 text-red-400 transition" title="Cancel reminder">
              <i data-lucide="trash" class="w-3.5 h-3.5"></i>
            </button>
          </td>
        </tr>`;
      })
      .join("");

    if (window.lucide) lucide.createIcons({ root: tbody });
  } catch (e) {
    console.error(e);
  }
}

window.deleteReminder = async (id) => {
  try {
    await fetch(`/api/reminders/${encodeURIComponent(id)}`, { method: "DELETE" });
    showToast("Reminder cancelled", "info");
    fetchRemindersData();
  } catch (e) {
    showToast("Failed to cancel reminder", "error");
  }
};

// ---------------------------------------------------------------------------
// 14. DIRECT MESSENGER
// ---------------------------------------------------------------------------
async function populateMessengerRecipients() {
  try {
    const res = await fetch("/api/chats");
    const chats = await res.json();
    const select = document.getElementById("directRecipientSelect");
    if (!select) return;

    select.innerHTML = `<option value="">-- Choose from known contacts or type below --</option>` +
      chats.map((c) => `<option value="${c.jid}">${escapeHtml(c.name)} (${c.phone || c.jid})</option>`).join("");
  } catch (e) {}
}

async function sendDirectMessage() {
  const jid = document.getElementById("directJidInput").value.trim();
  const text = document.getElementById("directMsgInput").value.trim();

  if (!jid || !text) return showToast("Please enter both recipient and message text", "error");

  try {
    const res = await fetch("/api/message/send", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ jid, text }),
    });
    const data = await res.json();

    if (data.success) {
      showToast(`Message successfully sent to ${jid}!`, "success");
      document.getElementById("directMsgInput").value = "";
    } else {
      showToast(`Send failed: ${data.error}`, "error");
    }
  } catch (err) {
    showToast(`Send error: ${err.message}`, "error");
  }
}

// ---------------------------------------------------------------------------
// 15. LIVE SSE TERMINAL LOGS WITH FILTERING
// ---------------------------------------------------------------------------
function initSseLogStream() {
  const evtSource = new EventSource("/api/logs/stream");

  evtSource.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      if (data.type === "INIT" && Array.isArray(data.logs)) {
        allTerminalLogs = data.logs;
      } else if (data.message) {
        allTerminalLogs.push(data);
      }
      renderFilteredLogs();
    } catch (e) {}
  };

  evtSource.onerror = () => {
    evtSource.close();
    setTimeout(initSseLogStream, 3000);
  };
}

function renderFilteredLogs() {
  const output = document.getElementById("terminalOutput");
  if (!output) return;

  const filter = document.getElementById("logTypeFilter")?.value || "all";
  let logs = allTerminalLogs;

  if (filter !== "all") {
    logs = logs.filter((l) => l.type === filter);
  }

  const badge = document.getElementById("logBadge");
  if (badge) badge.textContent = logs.length.toString();

  output.innerHTML = logs
    .slice(-100)
    .reverse()
    .map((entry) => {
      const time = entry.timestamp ? new Date(entry.timestamp).toLocaleTimeString() : "";
      const typeColors = {
        MSG: "text-emerald-400",
        CMD: "text-amber-400",
        AI: "text-purple-400",
        SYSTEM: "text-indigo-400",
        ERROR: "text-red-400",
        SUCCESS: "text-teal-400",
      };
      const color = typeColors[entry.type] || "text-slate-400";

      return `
      <div class="py-1 border-b border-slate-900/60 leading-relaxed font-mono">
        <span class="text-slate-600 select-none">[${time}]</span>
        <span class="font-bold ${color} px-1">[${entry.type}]</span>
        <span class="text-slate-300 font-sans">${escapeHtml(entry.message)}</span>
      </div>`;
    })
    .join("");
}

function escapeHtml(str) {
  return (str || "").replace(/[&<>"']/g, (m) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[m]));
}

// ---------------------------------------------------------------------------
// 16. MODALS MANAGEMENT (NICKNAME, MUTE, REMINDER, PERSONA, NOTE)
// ---------------------------------------------------------------------------
function setupModals() {
  // Rename Modal
  window.openRenameModal = (jid, currentName) => {
    document.getElementById("renameJidInput").value = jid;
    document.getElementById("renameNameInput").value = currentName || "";
    document.getElementById("renameModal").classList.remove("hidden");
    document.getElementById("renameNameInput").focus();
  };

  const closeRename = () => document.getElementById("renameModal")?.classList.add("hidden");
  document.getElementById("closeRenameModalBtn")?.addEventListener("click", closeRename);
  document.getElementById("cancelRenameModalBtn")?.addEventListener("click", closeRename);

  document.getElementById("saveRenameBtn")?.addEventListener("click", async () => {
    const jid = document.getElementById("renameJidInput").value;
    const name = document.getElementById("renameNameInput").value.trim();
    if (!name) return showToast("Please enter a name or nickname", "error");

    try {
      await fetch("/api/contacts/rename", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jid, name }),
      });

      showToast("Contact name updated!", "success");
      closeRename();
      fetchChatsData();
    } catch (e) {
      showToast("Failed to rename contact", "error");
    }
  });

  // Mute Modal
  document.getElementById("openMuteModalBtn")?.addEventListener("click", async () => {
    const res = await fetch("/api/chats");
    const chats = await res.json();
    const select = document.getElementById("muteChatSelect");
    if (select) {
      select.innerHTML = chats.map((c) => `<option value="${c.jid}">${escapeHtml(c.name)} (${c.phone || c.jid})</option>`).join("");
    }
    document.getElementById("muteModal")?.classList.remove("hidden");
  });

  const closeMute = () => document.getElementById("muteModal")?.classList.add("hidden");
  document.getElementById("closeMuteModalBtn")?.addEventListener("click", closeMute);
  document.getElementById("cancelMuteModalBtn")?.addEventListener("click", closeMute);

  document.getElementById("confirmMuteBtn")?.addEventListener("click", async () => {
    const jid = document.getElementById("muteChatSelect").value;
    const senderId = document.getElementById("muteSenderInput").value.trim();
    if (!senderId) return showToast("Please enter phone number or sender ID to mute", "error");

    try {
      await fetch("/api/moderation/mute", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jid, senderId }),
      });

      showToast("User successfully muted in chat", "success");
      closeMute();
      fetchMutesData();
    } catch (e) {
      showToast("Failed to mute user", "error");
    }
  });

  // Reminder Modal
  document.getElementById("openNewReminderModalBtn")?.addEventListener("click", async () => {
    const res = await fetch("/api/chats");
    const chats = await res.json();
    const select = document.getElementById("reminderChatSelect");
    if (select) {
      select.innerHTML = chats.map((c) => `<option value="${c.jid}">${escapeHtml(c.name)} (${c.phone || c.jid})</option>`).join("");
    }
    document.getElementById("reminderModal")?.classList.remove("hidden");
  });

  const closeReminder = () => document.getElementById("reminderModal")?.classList.add("hidden");
  document.getElementById("closeReminderModalBtn")?.addEventListener("click", closeReminder);
  document.getElementById("cancelReminderModalBtn")?.addEventListener("click", closeReminder);

  document.getElementById("saveReminderBtn")?.addEventListener("click", async () => {
    const jid = document.getElementById("reminderChatSelect").value;
    const minutes = document.getElementById("reminderMinutesInput").value;
    const text = document.getElementById("reminderTextInput").value.trim();

    if (!text || !minutes) return showToast("Please enter text and duration", "error");

    try {
      await fetch("/api/reminders", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jid, minutes, text }),
      });

      showToast("Reminder scheduled successfully!", "success");
      closeReminder();
      fetchRemindersData();
    } catch (e) {
      showToast("Failed to schedule reminder", "error");
    }
  });

  // Custom Persona Modal
  document.getElementById("openNewPersonaModalBtn")?.addEventListener("click", () => {
    document.getElementById("personaModal")?.classList.remove("hidden");
  });

  const closePersona = () => document.getElementById("personaModal")?.classList.add("hidden");
  document.getElementById("closePersonaModalBtn")?.addEventListener("click", closePersona);
  document.getElementById("cancelPersonaModalBtn")?.addEventListener("click", closePersona);

  document.getElementById("saveNewPersonaBtn")?.addEventListener("click", async () => {
    const key = document.getElementById("newPersonaKey").value.trim().toLowerCase();
    const label = document.getElementById("newPersonaLabel").value.trim();
    const flavor = document.getElementById("newPersonaFlavor").value.trim();

    if (!key || !label || !flavor) return showToast("Please fill all persona fields", "error");

    try {
      await fetch("/api/personas", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key, label, flavor }),
      });

      showToast("Persona added successfully!", "success");
      closePersona();
      fetchPersonasData();
    } catch (e) {
      showToast("Failed to save persona", "error");
    }
  });

  // Note Modal
  document.getElementById("openNewNoteModalBtn")?.addEventListener("click", () => {
    document.getElementById("noteModal")?.classList.remove("hidden");
    document.getElementById("newNotePath")?.focus();
  });

  const closeNote = () => document.getElementById("noteModal")?.classList.add("hidden");
  document.getElementById("closeNoteModalBtn")?.addEventListener("click", closeNote);
  document.getElementById("cancelNoteModalBtn")?.addEventListener("click", closeNote);

  document.getElementById("saveNewNoteBtn")?.addEventListener("click", async () => {
    const path = document.getElementById("newNotePath").value.trim();
    const content = document.getElementById("newNoteContent").value.trim();

    if (!path || !content) return showToast("Please enter path and note content", "error");

    try {
      await fetch("/api/notes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path, content }),
      });

      showToast(`Note saved: ${path}`, "success");
      document.getElementById("newNotePath").value = "";
      document.getElementById("newNoteContent").value = "";
      closeNote();
      fetchNotesData();
    } catch (e) {
      showToast("Failed to save note", "error");
    }
  });
}

// ---------------------------------------------------------------------------
// 17. COMMAND CHEAT SHEET & QUICK COPY
// ---------------------------------------------------------------------------
const ALL_BOT_COMMANDS = [
  // Superpower Tools
  { name: "!ocr", cat: "tools", syntax: "!ocr (reply to image)", desc: "Extracts text from handwriting, documents & photos using Gemini Vision." },
  { name: "!sum", cat: "tools", syntax: "!sum / !summarize (reply to long text)", desc: "Generates a clean 3-bullet executive summary + takeaway." },
  { name: "!tr", cat: "tools", syntax: "!tr <lang> [text]", desc: "Translates text into any language (e.g. !tr hi, !tr es, !tr ja)." },
  { name: "!poll", cat: "tools", syntax: '!poll "Question" "Option 1" "Option 2"', desc: "Creates native WhatsApp interactive voting polls." },
  { name: "!calc", cat: "tools", syntax: "!calc <math / currency / units>", desc: "Scientific calculator & currency conversion (e.g. 15% of $4500)." },
  { name: "!weather", cat: "tools", syntax: "!weather <city name>", desc: "Live weather forecast card (temp, humidity, wind & conditions)." },
  { name: "!define", cat: "tools", syntax: "!define <word>", desc: "Dictionary definition, pronunciation, synonyms and example sentence." },
  { name: "!tagall", cat: "tools", syntax: "!tagall <announcement>", desc: "Mentions all group members for important updates (Admin only)." },
  { name: "!quote", cat: "tools", syntax: "!quote / !motivation", desc: "Daily inspirational quote and thought." },
  { name: "!fact", cat: "tools", syntax: "!fact / !mindblown", desc: "Verified fascinating science or historical trivia fact." },
  { name: "!meme", cat: "tools", syntax: "!meme <top> | <bottom> (reply to image)", desc: "Creates authentic classic meme graphic from photo." },

  // Media & Downloads
  { name: "!s", cat: "media", syntax: "!s / !sticker (reply/attach media)", desc: "Converts image, video or GIF into WhatsApp WebP sticker in <1s." },
  { name: "!toimg", cat: "media", syntax: "!toimg (reply to sticker)", desc: "Converts static sticker back into high-resolution photo." },
  { name: "!togif", cat: "media", syntax: "!togif / !tovideo (reply to sticker)", desc: "Converts animated sticker into MP4 video / GIF." },
  { name: "!toaudio", cat: "media", syntax: "!toaudio / !tomp3 (reply to video)", desc: "Extracts audio from video file as 44.1kHz MP3." },
  { name: "!play", cat: "media", syntax: "!play <song name or link>", desc: "Downloads MP3 audio with embedded ID3 tags and album cover." },
  { name: "!song", cat: "media", syntax: "!song <query>", desc: "Instant audio search and stream on WhatsApp." },
  { name: "!video", cat: "media", syntax: "!video <link> / !mp4 <link>", desc: "Universal video downloader (YouTube, Instagram, Reddit, TikTok, X)." },
  { name: "!ig", cat: "media", syntax: "!ig <Instagram reel/post link>", desc: "Downloads Instagram Reels and video posts in HD." },
  { name: "!reddit", cat: "media", syntax: "!reddit <link>", desc: "Downloads Reddit video clips." },
  { name: "!yt", cat: "media", syntax: "!yt <search query>", desc: "Searches YouTube and displays top 5 clickable results." },
  { name: "!img", cat: "media", syntax: "!img <search query>", desc: "Fetches high-resolution images from Google search." },
  { name: "!vv", cat: "media", syntax: "!vv / !viewonce", desc: "Instantly intercepts and restores disappearing View-Once photos/videos." },
  { name: "!deleted", cat: "media", syntax: "!deleted list", desc: "Recovers revoked / deleted WhatsApp messages." },

  // AI & Personas
  { name: "!catchup", cat: "ai", syntax: "!catchup [50-100] / !recap", desc: "Summarizes missed texts and key discussions while you were away using AI." },
  { name: "!zero2", cat: "ai", syntax: "!zero2 <prompt>", desc: "Chat with Zero Two (darling, sweet, romantic personality)." },
  { name: "!makima", cat: "ai", syntax: "!makima <prompt>", desc: "Chat with Makima (dominant, calculated, cold charisma)." },
  { name: "!kurumi", cat: "ai", syntax: "!kurumi <prompt>", desc: "Chat with Kurumi Tokisaki (playful, seductive, gothic spirit)." },
  { name: "!mai", cat: "ai", syntax: "!mai <prompt>", desc: "Chat with Mai Sakurajima (teasing, mature, composed)." },
  { name: "!ganyu", cat: "ai", syntax: "!ganyu <prompt>", desc: "Chat with Ganyu (gentle, diligent, soft spoken)." },
  { name: "!johan", cat: "ai", syntax: "!johan <prompt>", desc: "Chat with Johan Liebert (philosophical, eerie, psychological master)." },
  { name: "!light", cat: "ai", syntax: "!light <prompt>", desc: "Chat with Light Yagami (righteous god complex, supreme intellect)." },
  { name: "!gojo", cat: "ai", syntax: "!gojo <prompt>", desc: "Chat with Satoru Gojo (overconfident, funny, strongest sorcerer)." },
  { name: "!sukuna", cat: "ai", syntax: "!sukuna <prompt>", desc: "Chat with Ryomen Sukuna (arrogant king of curses)." },
  { name: "!dazai", cat: "ai", syntax: "!dazai <prompt>", desc: "Chat with Osamu Dazai (witty, dramatic, eccentric genius)." },
  { name: "!tts", cat: "ai", syntax: "!tts <voice> <text>", desc: "Generates realistic text-to-speech audio in character voices." },
  { name: "!vc", cat: "ai", syntax: "!vc <model> [preset]", desc: "RVC AI Voice Conversion of voice notes and song covers." },
  { name: "!gen", cat: "ai", syntax: "!gen <prompt>", desc: "AI image generation for realistic and digital artwork." },
  { name: "!anime", cat: "ai", syntax: "!anime <prompt>", desc: "AI anime artwork generation." },

  // Admin & Controls
  { name: "!autoreply", cat: "admin", syntax: "!autoreply on [mode] / off", desc: "Smart Away & Auto-Reply bot with keyword triggers & visitor inbox." },
  { name: "!welcome", cat: "admin", syntax: "!welcome on/off / !welcome set <text>", desc: "Group welcomer greeting banner with custom template tags." },
  { name: "!mute", cat: "admin", syntax: "!mute @user / !unmute @user", desc: "Silences spam users; auto-deletes their messages instantly." },
  { name: "!remind", cat: "admin", syntax: "!remind <10m|1h> <reminder text>", desc: "Schedules automated timer reminder in chat." },
  { name: "!notes", cat: "admin", syntax: "!notes / !addnote <folder/file> <text>", desc: "Virtual file tree system for saving notes, media and code snippets." },
  { name: "!antidelete", cat: "admin", syntax: "!antidelete", desc: "Toggles anti-delete message interception per chat." },
  { name: "!features", cat: "admin", syntax: "!features on/off", desc: "Enables/disables interactive features and personas in group chats." },
  { name: "!storage", cat: "admin", syntax: "!storage / !storage clear media", desc: "Monitors disk usage and cleans cached media/stickers." },
  { name: "!status", cat: "admin", syntax: "!status / !botstatus / !uptime", desc: "Displays bot uptime, RAM usage, and connection status." },
  { name: "!wake", cat: "admin", syntax: "!wake / !sleep", desc: "Manually triggers low-power Sleep Mode or instant Wakeup." },
];

function renderCommandCheatSheet() {
  const query = (document.getElementById("commandSearchInput")?.value || "").toLowerCase().trim();
  const category = document.getElementById("commandCategoryFilter")?.value || "all";
  const grid = document.getElementById("commandsCardsGrid");
  if (!grid) return;

  const filtered = ALL_BOT_COMMANDS.filter((cmd) => {
    if (category !== "all" && cmd.cat !== category) return false;
    if (query) {
      const matchName = cmd.name.toLowerCase().includes(query);
      const matchSyntax = cmd.syntax.toLowerCase().includes(query);
      const matchDesc = cmd.desc.toLowerCase().includes(query);
      return matchName || matchSyntax || matchDesc;
    }
    return true;
  });

  const catBadges = {
    tools: { label: "Tool", color: "bg-teal-500/10 text-teal-400 border-teal-500/20" },
    media: { label: "Media", color: "bg-pink-500/10 text-pink-400 border-pink-500/20" },
    ai: { label: "AI", color: "bg-purple-500/10 text-purple-400 border-purple-500/20" },
    games: { label: "Game", color: "bg-amber-500/10 text-amber-400 border-amber-500/20" },
    admin: { label: "Admin", color: "bg-indigo-500/10 text-indigo-400 border-indigo-500/20" },
  };

  if (filtered.length === 0) {
    grid.innerHTML = `<div class="col-span-full py-12 text-center text-slate-500 font-sans">No matching commands found.</div>`;
    return;
  }

  grid.innerHTML = filtered
    .map((cmd) => {
      const badge = catBadges[cmd.cat] || catBadges.tools;
      return `
      <div class="p-4 rounded-2xl glass-card-interactive space-y-2 flex flex-col justify-between">
        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <span class="font-bold text-white font-mono text-sm">${cmd.name}</span>
            <span class="px-2 py-0.5 rounded text-[10px] font-mono border ${badge.color}">${badge.label}</span>
          </div>
          <div class="p-2 rounded-lg bg-dark-900/60 font-mono text-[11px] text-emerald-400 select-all border border-slate-800/60">
            ${escapeHtml(cmd.syntax)}
          </div>
          <p class="text-xs text-slate-400 font-sans leading-relaxed">${escapeHtml(cmd.desc)}</p>
        </div>
        <div class="pt-2 flex justify-end">
          <button onclick="copyToClipboard('${cmd.syntax.replace(/'/g, "\\'")}')" class="px-2.5 py-1 rounded-lg bg-dark-700 hover:bg-dark-600 text-slate-300 text-xs flex items-center gap-1.5 transition">
            <i data-lucide="copy" class="w-3 h-3 text-slate-400"></i> Copy
          </button>
        </div>
      </div>`;
    })
    .join("");

  if (window.lucide) lucide.createIcons({ root: grid });
}

window.copyToClipboard = (text) => {
  navigator.clipboard.writeText(text);
  showToast("Command copied to clipboard!", "success");
};
