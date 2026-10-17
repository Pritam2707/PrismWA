
(function () {
  "use strict";

  // Application State
  const state = {
    currentTab: "overview",
    theme: localStorage.getItem("prism_theme") || "dark",
    isSleeping: false,
    autoScrollLogs: true,
    logsFilter: "ALL",
    chats: [],
    logs: [],
    personas: {},
    welcomeConfig: {},
    mutes: [],
    reminders: [],
    aiConfig: null,
    meowcaller: null,
    selectedDurationMinutes: 30,
    pollTimer: null,
    logTimer: null,
  };

  // DOM Elements Cache
  const el = {
    appSidebar: document.getElementById("appSidebar"),
    mobileMenuBtn: document.getElementById("mobileMenuBtn"),
    pageTitle: document.getElementById("pageTitle"),
    btnToggleTheme: document.getElementById("btnToggleTheme"),
    themeIcon: document.getElementById("themeIcon"),
    btnToggleSleep: document.getElementById("btnToggleSleep"),
    sleepBtnIcon: document.getElementById("sleepBtnIcon"),
    sleepBtnText: document.getElementById("sleepBtnText"),
    btnQuickRefresh: document.getElementById("btnQuickRefresh"),
    btnOpenSendMessageModal: document.getElementById("btnOpenSendMessageModal"),
    toastContainer: document.getElementById("toastContainer"),

    // Overview Elements
    footerStatusDot: document.getElementById("footerStatusDot"),
    footerStatusText: document.getElementById("footerStatusText"),
    connectionNoticeCard: document.getElementById("connectionNoticeCard"),
    qrContainer: document.getElementById("qrContainer"),
    qrImage: document.getElementById("qrImage"),
    connHeadline: document.getElementById("connHeadline"),
    connDesc: document.getElementById("connDesc"),
    btnRefreshStatus: document.getElementById("btnRefreshStatus"),
    metricConnState: document.getElementById("metricConnState"),
    metricPhone: document.getElementById("metricPhone"),
    metricUptime: document.getElementById("metricUptime"),
    metricGoroutines: document.getElementById("metricGoroutines"),
    metricMemory: document.getElementById("metricMemory"),
    metricSysMemory: document.getElementById("metricSysMemory"),
    metricMessages: document.getElementById("metricMessages"),
    badgePowerState: document.getElementById("badgePowerState"),
    labelAntiDeleteCount: document.getElementById("labelAntiDeleteCount"),
    labelFeaturesCount: document.getElementById("labelFeaturesCount"),
    rtPlatform: document.getElementById("rtPlatform"),
    rtGoVersion: document.getElementById("rtGoVersion"),
    rtCpuCount: document.getElementById("rtCpuCount"),
    rtNumGC: document.getElementById("rtNumGC"),

    // Audit Vault
    auditTableBody: document.getElementById("auditTableBody"),
    btnRefreshAudit: document.getElementById("btnRefreshAudit"),
    btnClearAudit: document.getElementById("btnClearAudit"),

    // Chats
    searchChatsInput: document.getElementById("searchChatsInput"),
    chatsTableBody: document.getElementById("chatsTableBody"),

    // Moderation
    mutesTableBody: document.getElementById("mutesTableBody"),
    btnOpenMuteModal: document.getElementById("btnOpenMuteModal"),
    welcomeStatusBadge: document.getElementById("welcomeStatusBadge"),
    welcomeTemplateInput: document.getElementById("welcomeTemplateInput"),
    btnSaveWelcome: document.getElementById("btnSaveWelcome"),
    btnToggleWelcome: document.getElementById("btnToggleWelcome"),

    // Personas & AI Engine
    aiGlobalActiveBadge: document.getElementById("aiGlobalActiveBadge"),
    aiProviderGrid: document.getElementById("aiProviderGrid"),
    aiGlobalModelInput: document.getElementById("aiGlobalModelInput"),
    btnSaveGlobalAI: document.getElementById("btnSaveGlobalAI"),
    aiModelPresets: document.getElementById("aiModelPresets"),
    aiStyleGrid: document.getElementById("aiStyleGrid"),
    aiOverrideChatSelect: document.getElementById("aiOverrideChatSelect"),
    aiOverrideProviderSelect: document.getElementById("aiOverrideProviderSelect"),
    aiOverrideStyleSelect: document.getElementById("aiOverrideStyleSelect"),
    aiOverrideModelInput: document.getElementById("aiOverrideModelInput"),
    btnSaveChatOverride: document.getElementById("btnSaveChatOverride"),
    aiChatOverridesContainer: document.getElementById("aiChatOverridesContainer"),
    personasListContainer: document.getElementById("personasListContainer"),
    playgroundPersonaSelect: document.getElementById("playgroundPersonaSelect"),
    playgroundPromptInput: document.getElementById("playgroundPromptInput"),
    btnTestPersona: document.getElementById("btnTestPersona"),
    playgroundOutput: document.getElementById("playgroundOutput"),

    // Auto-Reply
    badgeAutoReplyState: document.getElementById("badgeAutoReplyState"),
    selectAutoReplyMode: document.getElementById("selectAutoReplyMode"),
    autoReplyDefaultText: document.getElementById("autoReplyDefaultText"),
    btnSaveAutoReply: document.getElementById("btnSaveAutoReply"),
    btnAddTriggerBtn: document.getElementById("btnAddTriggerBtn"),
    triggersTableBody: document.getElementById("triggersTableBody"),

    // Reminders
    remindersTableBody: document.getElementById("remindersTableBody"),
    btnOpenReminderModal: document.getElementById("btnOpenReminderModal"),

    // Notes
    notesTreeContainer: document.getElementById("notesTreeContainer"),

    // Storage
    storageMediaSize: document.getElementById("storageMediaSize"),
    storageDbSize: document.getElementById("storageDbSize"),
    storageNotesSize: document.getElementById("storageNotesSize"),
    storageAuditSize: document.getElementById("storageAuditSize"),
    btnCleanMedia: document.getElementById("btnCleanMedia"),

    // Logs
    logTerminal: document.getElementById("logTerminal"),
    logLevelFilter: document.getElementById("logLevelFilter"),
    btnToggleAutoScroll: document.getElementById("btnToggleAutoScroll"),
    btnClearLogs: document.getElementById("btnClearLogs"),

    // Modals
    btnOpenHelpModal: document.getElementById("btnOpenHelpModal"),
    modalHelp: document.getElementById("modalHelp"),
    helpSearchInput: document.getElementById("helpSearchInput"),
    helpFilterPills: document.getElementById("helpFilterPills"),
    helpCommandsList: document.getElementById("helpCommandsList"),

    modalSendMessage: document.getElementById("modalSendMessage"),
    sendMsgJid: document.getElementById("sendMsgJid"),
    sendMsgContent: document.getElementById("sendMsgContent"),
    btnSubmitSendMessage: document.getElementById("btnSubmitSendMessage"),

    modalMute: document.getElementById("modalMute"),
    muteTargetInput: document.getElementById("muteTargetInput"),
    btnSubmitMute: document.getElementById("btnSubmitMute"),

    modalReminder: document.getElementById("modalReminder"),
    reminderJidInput: document.getElementById("reminderJidInput"),
    reminderDurationInput: document.getElementById("reminderDurationInput"),
    reminderTextInput: document.getElementById("reminderTextInput"),
    btnSubmitReminder: document.getElementById("btnSubmitReminder"),

    modalTrigger: document.getElementById("modalTrigger"),
    triggerKeywordInput: document.getElementById("triggerKeywordInput"),
    triggerResponseInput: document.getElementById("triggerResponseInput"),
    btnSubmitTrigger: document.getElementById("btnSubmitTrigger"),

    // Reports & Deadlines
    reportDatePicker: document.getElementById("reportDatePicker"),
    btnGenerateReportSummary: document.getElementById("btnGenerateReportSummary"),
    reportSummaryStatusBadge: document.getElementById("reportSummaryStatusBadge"),
    reportSummaryContent: document.getElementById("reportSummaryContent"),
    reportsCountBadge: document.getElementById("reportsCountBadge"),
    reportsTableBody: document.getElementById("reportsTableBody"),
    newReportGroupInput: document.getElementById("newReportGroupInput"),
    btnAddReportGroupBtn: document.getElementById("btnAddReportGroupBtn"),
    reportGroupsList: document.getElementById("reportGroupsList"),
    leaderboardMiniBody: document.getElementById("leaderboardMiniBody"),
    pfpGroupJidInput: document.getElementById("pfpGroupJidInput"),
    pfpDateInput: document.getElementById("pfpDateInput"),
    pfpTitleInput: document.getElementById("pfpTitleInput"),
    btnSaveDeadline: document.getElementById("btnSaveDeadline"),
    deadlinesTableBody: document.getElementById("deadlinesTableBody"),
    pfpLivePreviewImg: document.getElementById("pfpLivePreviewImg"),

    // MeowCaller VoIP & Song Party
    sidebarCallerBadge: document.getElementById("sidebarCallerBadge"),
    labelMeowCallerOverview: document.getElementById("labelMeowCallerOverview"),
    badgeMeowCallerState: document.getElementById("badgeMeowCallerState"),
    btnQuickOpenCaller: document.getElementById("btnQuickOpenCaller"),
    callerStatusBadge: document.getElementById("callerStatusBadge"),
    callerTimerRemainingBadge: document.getElementById("callerTimerRemainingBadge"),
    btnToggleCallerMaster: document.getElementById("btnToggleCallerMaster"),
    callerMasterBtnIcon: document.getElementById("callerMasterBtnIcon"),
    callerMasterBtnText: document.getElementById("callerMasterBtnText"),
    callerDurationPresets: document.getElementById("callerDurationPresets"),
    checkCallerAutoAccept: document.getElementById("checkCallerAutoAccept"),
    callerCallPhaseBadge: document.getElementById("callerCallPhaseBadge"),
    callerCallDuration: document.getElementById("callerCallDuration"),
    callerAvatarIcon: document.getElementById("callerAvatarIcon"),
    callerCallPeer: document.getElementById("callerCallPeer"),
    callerCallSubtext: document.getElementById("callerCallSubtext"),
    callerDialTarget: document.getElementById("callerDialTarget"),
    callerRecentRecipients: document.getElementById("callerRecentRecipients"),
    callerIncomingAlert: document.getElementById("callerIncomingAlert"),
    callerIncomingDuration: document.getElementById("callerIncomingDuration"),
    callerIncomingPeer: document.getElementById("callerIncomingPeer"),
    btnCallerAnswer: document.getElementById("btnCallerAnswer"),
    btnCallerReject: document.getElementById("btnCallerReject"),
    btnCallerDial: document.getElementById("btnCallerDial"),
    btnCallerHangup: document.getElementById("btnCallerHangup"),
    partyVisualizer: document.getElementById("partyVisualizer"),
    partyPlaybackBadge: document.getElementById("partyPlaybackBadge"),
    partyNowPlayingTitle: document.getElementById("partyNowPlayingTitle"),
    partyNowPlayingRequester: document.getElementById("partyNowPlayingRequester"),
    btnPartyPauseResume: document.getElementById("btnPartyPauseResume"),
    partyPauseResumeIcon: document.getElementById("partyPauseResumeIcon"),
    partyPauseResumeText: document.getElementById("partyPauseResumeText"),
    btnPartySkip: document.getElementById("btnPartySkip"),
    btnPartyStop: document.getElementById("btnPartyStop"),
    partySongInput: document.getElementById("partySongInput"),
    btnPartyQueueSong: document.getElementById("btnPartyQueueSong"),
    partyQueueCountBadge: document.getElementById("partyQueueCountBadge"),
    partyQueueTableBody: document.getElementById("partyQueueTableBody"),

    // Realtime AI Voice Chatbot
    voiceBotLiveDot: document.getElementById("voiceBotLiveDot"),
    voiceBotStatusBadge: document.getElementById("voiceBotStatusBadge"),
    voiceBotLatencyBadge: document.getElementById("voiceBotLatencyBadge"),
    btnToggleVoiceBot: document.getElementById("btnToggleVoiceBot"),
    voiceBotBtnIcon: document.getElementById("voiceBotBtnIcon"),
    voiceBotBtnText: document.getElementById("voiceBotBtnText"),
    voiceBotServerUrl: document.getElementById("voiceBotServerUrl"),
    btnConnectVoiceBotWs: document.getElementById("btnConnectVoiceBotWs"),
    btnConnectWsIcon: document.getElementById("btnConnectWsIcon"),
    btnConnectWsText: document.getElementById("btnConnectWsText"),
    voiceBotWsConnBadge: document.getElementById("voiceBotWsConnBadge"),
    voiceBotWsError: document.getElementById("voiceBotWsError"),
    voiceBotVoiceSelect: document.getElementById("voiceBotVoiceSelect"),
    voiceBotModelSelect: document.getElementById("voiceBotModelSelect"),
    voiceBotSilenceSlider: document.getElementById("voiceBotSilenceSlider"),
    voiceBotSilenceVal: document.getElementById("voiceBotSilenceVal"),
    voiceBotPromptInput: document.getElementById("voiceBotPromptInput"),
    voiceBotSongTriggerInput: document.getElementById("voiceBotSongTriggerInput"),
    btnSaveVoiceBotConfig: document.getElementById("btnSaveVoiceBotConfig"),
    voiceBotAsrStateBadge: document.getElementById("voiceBotAsrStateBadge"),
    voiceBotLiveTranscript: document.getElementById("voiceBotLiveTranscript"),
    voiceBotTtsStateBadge: document.getElementById("voiceBotTtsStateBadge"),
    voiceBotLiveResponse: document.getElementById("voiceBotLiveResponse"),
    voiceBotMetricLatency: document.getElementById("voiceBotMetricLatency"),
    voiceBotMetricContext: document.getElementById("voiceBotMetricContext"),
    voiceBotMetricTurns: document.getElementById("voiceBotMetricTurns"),

    // Live Call Audio Monitor
    audioMonitorLiveDot: document.getElementById("audioMonitorLiveDot"),
    audioMonitorBadge: document.getElementById("audioMonitorBadge"),
    btnToggleAudioListen: document.getElementById("btnToggleAudioListen"),
    audioListenBtnIcon: document.getElementById("audioListenBtnIcon"),
    audioListenBtnText: document.getElementById("audioListenBtnText"),
    audioFeedSelector: document.getElementById("audioFeedSelector"),
    audioMonitorStats: document.getElementById("audioMonitorStats"),
    badgeIncomingVoiceActivity: document.getElementById("badgeIncomingVoiceActivity"),
    canvasIncomingAudio: document.getElementById("canvasIncomingAudio"),
    sliderGainIncoming: document.getElementById("sliderGainIncoming"),
    labelGainIncoming: document.getElementById("labelGainIncoming"),
    badgeOutgoingVoiceActivity: document.getElementById("badgeOutgoingVoiceActivity"),
    canvasOutgoingAudio: document.getElementById("canvasOutgoingAudio"),
    sliderGainOutgoing: document.getElementById("sliderGainOutgoing"),
    labelGainOutgoing: document.getElementById("labelGainOutgoing"),
    btnAudioMuteToggle: document.getElementById("btnAudioMuteToggle"),
    audioMuteIcon: document.getElementById("audioMuteIcon"),
    sliderMasterVolume: document.getElementById("sliderMasterVolume"),
    labelMasterVolume: document.getElementById("labelMasterVolume"),
    callerLiveAudioNotice: document.getElementById("callerLiveAudioNotice"),
    callerLiveAudioNoticeText: document.getElementById("callerLiveAudioNoticeText"),
    btnQuickStartListening: document.getElementById("btnQuickStartListening"),
    btnTestAudioChime: document.getElementById("btnTestAudioChime"),
    nativeAudioCallerStream: document.getElementById("nativeAudioCallerStream"),

    // Radio Station DJ & Live Mic
    radioLiveDot: document.getElementById("radioLiveDot"),
    radioOnAirBadge: document.getElementById("radioOnAirBadge"),
    radioSpeakingBadge: document.getElementById("radioSpeakingBadge"),
    btnRadioPTT: document.getElementById("btnRadioPTT"),
    radioPttText: document.getElementById("radioPttText"),
    btnRadioMicToggle: document.getElementById("btnRadioMicToggle"),
    radioMicBtnIcon: document.getElementById("radioMicBtnIcon"),
    radioMicBtnText: document.getElementById("radioMicBtnText"),
    radioModeSelector: document.getElementById("radioModeSelector"),
    selectRadioMicDevice: document.getElementById("selectRadioMicDevice"),
    sliderRadioMicGain: document.getElementById("sliderRadioMicGain"),
    labelRadioMicGain: document.getElementById("labelRadioMicGain"),
    sliderRadioDuckLevel: document.getElementById("sliderRadioDuckLevel"),
    labelRadioDuckLevel: document.getElementById("labelRadioDuckLevel"),
    radioVuBar: document.getElementById("radioVuBar"),
    labelRadioMicDb: document.getElementById("labelRadioMicDb"),
    radioStatusText: document.getElementById("radioStatusText"),
    radioMixFeedback: document.getElementById("radioMixFeedback"),
  };

  // ==========================================================================
  // Toast Notifications
  // ==========================================================================
  function showToast(message, type = "success") {
    if (!el.toastContainer) return;
    const toast = document.createElement("div");
    toast.className = `toast toast-${type}`;

    let icon = "✓";
    if (type === "danger") icon = "✕";
    if (type === "warning") icon = "!";

    toast.innerHTML = `<span style="font-weight:700;">${icon}</span> <span>${escapeHtml(message)}</span>`;
    el.toastContainer.appendChild(toast);

    setTimeout(() => {
      toast.style.opacity = "0";
      toast.style.transform = "translateY(6px)";
      toast.style.transition = "all 200ms ease";
      setTimeout(() => toast.remove(), 220);
    }, 3200);
  }

  function escapeHtml(str) {
    if (str === null || str === undefined) return "";
    return String(str)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  function formatTime(secs) {
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    const s = Math.floor(secs % 60);
    return [h, m, s].map((v) => String(v).padStart(2, "0")).join(":");
  }

  //theme
  function initTheme() {
    document.documentElement.setAttribute("data-theme", state.theme);
    updateThemeIcon();
  }

  function toggleTheme() {
    state.theme = state.theme === "dark" ? "light" : "dark";
    localStorage.setItem("prism_theme", state.theme);
    document.documentElement.setAttribute("data-theme", state.theme);
    updateThemeIcon();
  }

  function updateThemeIcon() {
    if (!el.themeIcon) return;
    el.themeIcon.innerHTML = `<use href="#icon-${state.theme === "dark" ? "sun" : "moon"}"/>`;
  }

  // Navigation & Tab Switching

  function switchTab(tabName) {
    state.currentTab = tabName;

    // Update active nav button
    document.querySelectorAll(".nav-btn").forEach((btn) => {
      if (btn.getAttribute("data-tab") === tabName) {
        btn.classList.add("active");
      } else {
        btn.classList.remove("active");
      }
    });

    // Update view visibility
    document.querySelectorAll(".tab-view").forEach((view) => {
      if (view.id === `view-${tabName}`) {
        view.classList.add("active");
      } else {
        view.classList.remove("active");
      }
    });

    // Update Title in Topbar
    const tabTitles = {
      overview: "Overview",
      audit: "Audit Vault (Deleted Messages)",
      chats: "Chats & Contacts",
      moderation: "Moderation Studio",
      personas: "AI Persona Studio",
      reports: "Daily Reports & Deadline PFP Manager",
      autoreply: "Auto-Reply Engine",
      reminders: "Reminders & Scheduler",
      notes: "Notes Explorer",
      meowcaller: "MeowCaller VoIP & Song Party Studio",
      storage: "Disk & Storage Utilization",
      logs: "Real-Time Console Logs",
    };
    if (el.pageTitle) {
      el.pageTitle.textContent = tabTitles[tabName] || "PrismBot Console";
    }

    // Close mobile menu if open
    if (el.appSidebar) {
      el.appSidebar.classList.remove("open");
    }

    // Load active tab data immediately
    loadCurrentTabData();
  }

  function loadCurrentTabData() {
    switch (state.currentTab) {
      case "overview":
        fetchStatus();
        break;
      case "audit":
        fetchAudit();
        break;
      case "chats":
        fetchChats();
        break;
      case "moderation":
        fetchMutes();
        fetchWelcome();
        break;
      case "personas":
        fetchPersonas();
        fetchAIConfig();
        break;
      case "reports":
        fetchReportsData();
        break;
      case "autoreply":
        fetchAutoReply();
        break;
      case "reminders":
        fetchReminders();
        break;
      case "notes":
        fetchNotes();
        break;
      case "meowcaller":
        fetchMeowCallerStatus();
        break;
      case "storage":
        fetchStorage();
        break;
      case "logs":
        fetchLogs();
        break;
    }
  }

  // Modals Manager
  function openModal(modalEl) {
    if (!modalEl) return;
    modalEl.classList.add("active");
  }

  function closeModal(modalEl) {
    if (!modalEl) return;
    modalEl.classList.remove("active");
  }

  function initModalListeners() {
    // Close buttons
    document.querySelectorAll("[data-close-modal]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        const modal = e.target.closest(".modal-overlay");
        if (modal) closeModal(modal);
      });
    });

    // Background backdrop click
    document.querySelectorAll(".modal-overlay").forEach((modal) => {
      modal.addEventListener("click", (e) => {
        if (e.target === modal) closeModal(modal);
      });
    });

    // Escape key closes modals (R-32)
    window.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        document.querySelectorAll(".modal-overlay.active").forEach((modal) => {
          closeModal(modal);
        });
      }
    });
  }

  // API Calls & Renderers
  // Status Telemetry
  async function fetchStatus() {
    try {
      const res = await fetch("/api/status");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      renderStatus(data);
    } catch (err) {
      if (el.footerStatusDot) {
        el.footerStatusDot.className = "status-dot offline";
      }
      if (el.footerStatusText) {
        el.footerStatusText.textContent = "Offline / Server Error";
      }
    }
  }

  function renderStatus(data) {
    const bot = data.bot || {};
    const sys = data.system || {};

    state.isSleeping = !!bot.isSleeping;

    // Power status
    if (el.badgePowerState) {
      el.badgePowerState.textContent = state.isSleeping ? "Sleeping" : "Awake";
      el.badgePowerState.className = `badge ${state.isSleeping ? "badge-warning" : "badge-success"}`;
    }
    if (el.sleepBtnIcon) el.sleepBtnIcon.innerHTML = state.isSleeping ? '<svg class="icon-inline"><use href="#icon-clock"/></svg>' : '<svg class="icon-inline"><use href="#icon-moon"/></svg>';
    if (el.sleepBtnText) el.sleepBtnText.textContent = state.isSleeping ? "Wake Up" : "Sleep";

    // Metrics
    const connState = bot.connectionState || "disconnected";
    if (el.metricConnState) el.metricConnState.textContent = capitalize(connState);
    if (el.metricPhone) {
      el.metricPhone.textContent = bot.user && bot.user.phone ? `+${bot.user.phone}` : "No active pairing";
    }
    if (el.metricUptime) el.metricUptime.textContent = formatTime(bot.uptimeSecs || 0);
    const allocVal = sys.allocMB !== undefined ? sys.allocMB : (sys.heapUsedMB !== undefined ? sys.heapUsedMB : 0);
    const sysVal = sys.sysMB !== undefined ? sys.sysMB : (sys.rssMB !== undefined ? sys.rssMB : 0);
    if (el.metricMemory) el.metricMemory.textContent = `${allocVal} MB`;
    if (el.metricSysMemory) el.metricSysMemory.textContent = `System OS: ${sysVal} MB`;
    if (el.metricMessages) el.metricMessages.textContent = bot.totalCachedMessages || 0;

    // Scope counters
    if (el.labelAntiDeleteCount) {
      el.labelAntiDeleteCount.textContent = `${bot.antiDeleteChatsCount || 0} chats protected`;
    }
    if (el.labelFeaturesCount) {
      el.labelFeaturesCount.textContent = `${bot.featureChatsCount || 0} chats enabled`;
    }

    // Diagnostics
    if (el.rtPlatform) el.rtPlatform.textContent = `${sys.platform || "-"} / ${sys.arch || "-"}`;
    if (el.rtGoVersion) el.rtGoVersion.textContent = sys.goVersion || "-";
    if (el.rtCpuCount) el.rtCpuCount.textContent = sys.cpuCount || "-";
    if (el.rtNumGC) el.rtNumGC.textContent = sys.numGC !== undefined ? sys.numGC : "-";

    // Footer indicator
    if (el.footerStatusDot) {
      if (connState === "connected" || connState === "open") {
        el.footerStatusDot.className = "status-dot online";
        if (el.footerStatusText) el.footerStatusText.textContent = "Online";
      } else if (connState === "qr_pending" || connState === "connecting") {
        el.footerStatusDot.className = "status-dot warning";
        if (el.footerStatusText) el.footerStatusText.textContent = "Awaiting QR";
      } else {
        el.footerStatusDot.className = "status-dot offline";
        if (el.footerStatusText) el.footerStatusText.textContent = "Disconnected";
      }
    }

    // QR / Pairing Card
    if (connState !== "connected" && connState !== "open") {
      if (el.connectionNoticeCard) el.connectionNoticeCard.style.display = "block";
      if (bot.qrCode) {
        if (el.qrContainer) el.qrContainer.style.display = "flex";
        if (el.qrImage) el.qrImage.src = bot.qrCode;
        if (el.connHeadline) el.connHeadline.textContent = "Pairing Required";
        if (el.connDesc) {
          el.connDesc.textContent = "Open WhatsApp on your mobile phone, navigate to Linked Devices, and scan the QR code.";
        }
      } else {
        if (el.qrContainer) el.qrContainer.style.display = "none";
        if (el.connHeadline) el.connHeadline.textContent = "Connecting to WhatsApp Network";
        if (el.connDesc) {
          el.connDesc.textContent = "PrismBot is opening the socket and acquiring device credentials.";
        }
      }
    } else {
      if (el.connectionNoticeCard) el.connectionNoticeCard.style.display = "none";
    }

    if (bot.meowcaller) {
      updateMeowCallerStatusUI(bot.meowcaller);
    }
  }

  function capitalize(str) {
    if (!str) return "";
    return str.charAt(0).toUpperCase() + str.slice(1);
  }

  // Toggle Sleep / Wake
  async function toggleSleep() {
    try {
      const res = await fetch("/api/control/toggle-sleep", { method: "POST" });
      const data = await res.json();
      if (data.success) {
        state.isSleeping = data.isSleeping;
        showToast(state.isSleeping ? "PrismBot is now in Sleep Mode" : "PrismBot is now Awake", "success");
        fetchStatus();
      }
    } catch (err) {
      showToast(`Action failed: ${err.message}`, "danger");
    }
  }

  // 2. Audit Vault
  async function fetchAudit() {
    if (!el.auditTableBody) return;
    try {
      const res = await fetch("/api/audit/deleted");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      renderAuditTable(data);
    } catch (err) {
      renderAuditEmpty(`Failed to load audit vault: ${err.message}`);
    }
  }

  function renderAuditTable(data) {
    if (!el.auditTableBody) return;
    const entries = [];

    // The backend returns { deletedMsgs: { "chatJID": [ ... ] }, deletedVo: { ... } }
    const grouped = (data && data.deletedMsgs) ? data.deletedMsgs : data;

    if (Array.isArray(grouped)) {
      grouped.forEach((m) => entries.push(m));
    } else if (typeof grouped === "object" && grouped !== null) {
      for (const chatJID in grouped) {
        const msgs = grouped[chatJID] || [];
        if (Array.isArray(msgs)) {
          msgs.forEach((m) => {
            if (!m.chatJID && !m.chatJid) {
              m.chatJID = chatJID;
            }
            entries.push(m);
          });
        }
      }
    }

    if (entries.length === 0) {
      renderAuditEmpty("No deleted messages have been intercepted yet.");
      return;
    }

    // Sort newest first by deletedAt or timestamp
    entries.sort((a, b) => {
      const timeA = new Date(a.deletedAt || a.timestamp || 0).getTime();
      const timeB = new Date(b.deletedAt || b.timestamp || 0).getTime();
      return timeB - timeA;
    });

    el.auditTableBody.innerHTML = entries
      .map((item) => {
        const dateObj = item.deletedAt ? new Date(item.deletedAt) : (item.timestamp ? new Date(item.timestamp * 1000) : null);
        const timeStr = dateObj ? dateObj.toLocaleString([], { dateStyle: "short", timeStyle: "medium" }) : "-";
        const sender = escapeHtml(item.senderName || item.pushName || item.senderJid || item.sender || "Unknown");
        const chat = escapeHtml(item.chatName || item.chatJid || item.chatJID || "Direct Message");
        const type = escapeHtml(item.mediaType || "text");
        const content = escapeHtml(item.text || item.caption || (item.localFilePath ? "Attached Media" : "Empty message"));

        let mediaLink = "";
        const filePath = item.localFilePath || item.mediaUrl;
        if (filePath) {
          const webPath = filePath.replace(/\\/g, "/");
          const cleanUrl = webPath.startsWith("/") ? webPath : "/" + webPath;
          mediaLink = `<a href="${escapeHtml(cleanUrl)}" target="_blank" rel="noopener" class="btn btn-sm" style="margin-left: 8px;">View File</a>`;
        }

        const msgId = item.id || item.messageId || "";
        const chatJid = item.chatJid || item.chatJID || item.chat || "";

        return `
          <tr>
            <td style="font-family: var(--font-mono); font-size: 11px; color: var(--text-muted); white-space: nowrap;">${timeStr}</td>
            <td>
              <div style="font-weight: 600;">${sender}</div>
              <div style="font-size: 11px; color: var(--text-muted); font-family: var(--font-mono);">${chat}</div>
            </td>
            <td><span class="badge badge-neutral">${type}</span></td>
            <td style="font-size: 12px; line-height: 1.4;">${content} ${mediaLink}</td>
            <td>
              <button class="btn btn-sm btn-danger btn-delete-audit" data-chat="${escapeHtml(chatJid)}" data-id="${escapeHtml(msgId)}">Delete</button>
            </td>
          </tr>
        `;
      })
      .join("");

    // Attach row delete listeners
    el.auditTableBody.querySelectorAll(".btn-delete-audit").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const chat = btn.getAttribute("data-chat");
        const id = btn.getAttribute("data-id");
        if (!id) return;
        try {
          const deleteUrl = chat ? `/api/audit/deleted/${encodeURIComponent(chat)}/${encodeURIComponent(id)}` : `/api/audit/deleted/${encodeURIComponent(id)}`;
          const res = await fetch(deleteUrl, { method: "DELETE" });
          if (res.ok) {
            showToast("Audit entry deleted", "success");
            fetchAudit();
          }
        } catch (err) {
          showToast("Failed to delete entry", "danger");
        }
      });
    });
  }

  function renderAuditEmpty(msg) {
    if (!el.auditTableBody) return;
    el.auditTableBody.innerHTML = `
      <tr>
        <td colspan="5">
          <div class="empty-state">
            <svg><use href="#icon-shield"/></svg>
            <div class="empty-title">Audit Vault Empty</div>
            <div class="empty-desc">${escapeHtml(msg)}</div>
          </div>
        </td>
      </tr>
    `;
  }

  // 3. Chats & Contacts
  async function fetchChats() {
    if (!el.chatsTableBody) return;
    try {
      const res = await fetch("/api/chats");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      state.chats = await res.json();
      renderChatsTable();
    } catch (err) {
      if (el.chatsTableBody) {
        el.chatsTableBody.innerHTML = `<tr><td colspan="6" style="text-align:center; padding: 24px;">Failed to load chats: ${escapeHtml(err.message)}</td></tr>`;
      }
    }
  }

  function renderChatsTable() {
    if (!el.chatsTableBody) return;
    const query = (el.searchChatsInput?.value || "").toLowerCase().trim();

    const filtered = state.chats.filter((c) => {
      if (!query) return true;
      const name = (c.name || "").toLowerCase();
      const jid = (c.jid || "").toLowerCase();
      const phone = (c.phone || "").toLowerCase();
      return name.includes(query) || jid.includes(query) || phone.includes(query);
    });

    if (filtered.length === 0) {
      el.chatsTableBody.innerHTML = `
        <tr>
          <td colspan="6">
            <div class="empty-state">
              <svg><use href="#icon-users"/></svg>
              <div class="empty-title">No Chats Found</div>
              <div class="empty-desc">No active chats match your filter criteria.</div>
            </div>
          </td>
        </tr>
      `;
      return;
    }

    el.chatsTableBody.innerHTML = filtered
      .map((c) => {
        const isGroup = !!c.isGroup;
        const name = escapeHtml(c.name || c.phone || "Unknown");
        const jid = escapeHtml(c.jid || "");
        const antiDelete = !!c.antiDelete;
        const features = !!c.featuresEnabled;

        return `
          <tr>
            <td>
              <div style="font-weight: 600;">${name}</div>
            </td>
            <td style="font-family: var(--font-mono); font-size: 11px; color: var(--text-secondary);">${jid}</td>
            <td>
              <span class="badge ${isGroup ? "badge-neutral" : "badge-success"}">${isGroup ? "Group" : "Direct Message"}</span>
            </td>
            <td>
              <button class="btn btn-sm ${antiDelete ? "btn-success" : ""}" data-toggle-antidelete="${jid}" data-state="${antiDelete}">
                ${antiDelete ? "Protected" : "Disabled"}
              </button>
            </td>
            <td>
              <button class="btn btn-sm ${features ? "btn-primary" : ""}" data-toggle-features="${jid}" data-state="${features}">
                ${features ? "Active" : "Disabled"}
              </button>
            </td>
            <td>
              <div style="display: flex; gap: 4px;">
                <button class="btn btn-sm btn-action-send" data-jid="${jid}">Send</button>
              </div>
            </td>
          </tr>
        `;
      })
      .join("");

    // Toggle Anti-Delete Listeners
    el.chatsTableBody.querySelectorAll("[data-toggle-antidelete]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const jid = btn.getAttribute("data-toggle-antidelete");
        const current = btn.getAttribute("data-state") === "true";
        try {
          const res = await fetch("/api/control/toggle-antidelete", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ jid, enable: !current }),
          });
          if (res.ok) {
            showToast(`Anti-delete ${!current ? "enabled" : "disabled"} for chat`, "success");
            fetchChats();
          }
        } catch (e) {
          showToast("Failed to toggle anti-delete", "danger");
        }
      });
    });

    // Toggle Features Listeners
    el.chatsTableBody.querySelectorAll("[data-toggle-features]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const jid = btn.getAttribute("data-toggle-features");
        const current = btn.getAttribute("data-state") === "true";
        try {
          const res = await fetch("/api/control/toggle-features", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ jid, enable: !current }),
          });
          if (res.ok) {
            showToast(`Interactive features ${!current ? "enabled" : "disabled"} for chat`, "success");
            fetchChats();
          }
        } catch (e) {
          showToast("Failed to toggle features", "danger");
        }
      });
    });

    // Send Message Button in Row
    el.chatsTableBody.querySelectorAll(".btn-action-send").forEach((btn) => {
      btn.addEventListener("click", () => {
        const jid = btn.getAttribute("data-jid");
        if (el.sendMsgJid) el.sendMsgJid.value = jid;
        openModal(el.modalSendMessage);
        if (el.sendMsgContent) el.sendMsgContent.focus();
      });
    });
  }

  // 4. Moderation Studio
  async function fetchMutes() {
    if (!el.mutesTableBody) return;
    try {
      const res = await fetch("/api/moderation/mutes");
      const list = await res.json();
      renderMutesTable(list);
    } catch (e) {
      if (el.mutesTableBody) {
        el.mutesTableBody.innerHTML = `<tr><td colspan="4" style="text-align:center; padding: 18px;">Failed to load mutes.</td></tr>`;
      }
    }
  }

  function renderMutesTable(list) {
    if (!el.mutesTableBody) return;
    if (!list || list.length === 0) {
      el.mutesTableBody.innerHTML = `
        <tr>
          <td colspan="4">
            <div class="empty-state">
              <svg><use href="#icon-slash"/></svg>
              <div class="empty-title">No Muted Users</div>
              <div class="empty-desc">No participants are currently silenced.</div>
            </div>
          </td>
        </tr>
      `;
      return;
    }

    el.mutesTableBody.innerHTML = list
      .map((item) => {
        const user = escapeHtml(item.senderName || item.phone || item.senderId || "Unknown");
        const jid = escapeHtml(item.jid || "");
        const senderId = escapeHtml(item.senderId || "");

        return `
          <tr>
            <td style="font-weight: 600;">${user}</td>
            <td style="font-size: 11px; color: var(--text-secondary);">${escapeHtml(item.chatName || jid)}</td>
            <td><span class="badge badge-danger">Silenced</span></td>
            <td>
              <button class="btn btn-sm btn-unmute" data-jid="${jid}" data-sender="${senderId}">Unmute</button>
            </td>
          </tr>
        `;
      })
      .join("");

    el.mutesTableBody.querySelectorAll(".btn-unmute").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const jid = btn.getAttribute("data-jid");
        const sender = btn.getAttribute("data-sender");
        try {
          const res = await fetch(`/api/moderation/mute/${encodeURIComponent(jid)}/${encodeURIComponent(sender)}`, {
            method: "DELETE",
          });
          if (res.ok) {
            showToast("User unmuted successfully", "success");
            fetchMutes();
          }
        } catch (e) {
          showToast("Failed to unmute user", "danger");
        }
      });
    });
  }

  async function fetchWelcome() {
    try {
      const res = await fetch("/api/welcome");
      state.welcomeConfig = await res.json();
      // Pick first group config or default
      const firstKey = Object.keys(state.welcomeConfig)[0];
      const cfg = firstKey ? state.welcomeConfig[firstKey] : null;

      if (cfg) {
        if (el.welcomeTemplateInput) el.welcomeTemplateInput.value = cfg.text || "";
        if (el.welcomeStatusBadge) {
          el.welcomeStatusBadge.textContent = cfg.enabled ? "Status: Active" : "Status: Disabled";
          el.welcomeStatusBadge.className = `badge ${cfg.enabled ? "badge-success" : "badge-neutral"}`;
        }
      }
    } catch (e) { }
  }

  // 5. AI Engine, Multi-Providers & Personas
  async function fetchAIConfig() {
    try {
      const res = await fetch("/api/ai/config");
      const data = await res.json();
      if (data.success) {
        state.aiConfig = data;
        renderAIProviderEngine();
        renderAIChatOverrides();
      }
    } catch (e) {
      console.error("Failed to load AI config", e);
    }
  }

  function renderAIProviderEngine() {
    if (!state.aiConfig) return;
    const { activeProvider, activeModel, status, recommendedModels } = state.aiConfig;

    if (el.aiGlobalActiveBadge) {
      el.aiGlobalActiveBadge.textContent = (activeProvider || "gemini").toUpperCase();
      el.aiGlobalActiveBadge.className = activeProvider === "openrouter" ? "badge badge-primary" : "badge badge-success";
    }

    if (el.aiGlobalModelInput && !el.aiGlobalModelInput.dataset.userEdited) {
      el.aiGlobalModelInput.value = activeModel || "";
    }

    const providers = [
      { id: "gemini", name: "Gemini", sub: "Standard / Moderated", isReady: status?.gemini },
      { id: "openrouter", name: "OpenRouter", sub: "Uncensored / Dolphin / Venice", isReady: status?.openrouter },
      { id: "groq", name: "GroqCloud", sub: "Ultra-fast Llama & GPT-OSS", isReady: status?.groq },
      { id: "ollama", name: "Ollama Local", sub: "100% Free / Unfiltered", isReady: status?.ollama },
    ];

    if (el.aiProviderGrid) {
      el.aiProviderGrid.innerHTML = providers.map(p => {
        const isCurrent = p.id === activeProvider;
        const border = isCurrent ? "var(--accent)" : "var(--border-default)";
        const bg = isCurrent ? "var(--accent-subtle)" : "var(--bg-subtle)";
        const statusBadge = p.isReady
          ? `<span style="font-size: 10px; color: var(--success-text); font-weight: 600;">● Ready</span>`
          : `<span style="font-size: 10px; color: var(--danger-text); font-weight: 600;">● Key Missing</span>`;

        return `
          <div class="provider-card" data-provider="${p.id}" style="padding: 10px; border: 2px solid ${border}; background: ${bg}; border-radius: var(--radius-sm); cursor: pointer; transition: all 0.15s ease;">
            <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 2px;">
              <span style="font-weight: 600; font-size: 13px; color: var(--text-primary);">${p.name}</span>
              ${statusBadge}
            </div>
            <div style="font-size: 11px; color: var(--text-secondary);">${p.sub}</div>
          </div>
        `;
      }).join("");

      el.aiProviderGrid.querySelectorAll(".provider-card").forEach(card => {
        card.addEventListener("click", async () => {
          const prov = card.dataset.provider;
          await setGlobalProvider(prov);
        });
      });
    }

    // Render model presets
    if (el.aiModelPresets) {
      const presets = recommendedModels?.[activeProvider] || [];
      el.aiModelPresets.innerHTML = presets.map(m => {
        const isSelected = m === activeModel;
        return `
          <button type="button" class="btn btn-secondary btn-sm preset-pill" data-model="${escapeHtml(m)}" style="font-size: 10px; padding: 2px 6px; ${isSelected ? 'border-color: var(--accent); color: var(--accent);' : ''}">
            ${escapeHtml(m.split("/").pop() || m)}
          </button>
        `;
      }).join("");

      el.aiModelPresets.querySelectorAll(".preset-pill").forEach(pill => {
        pill.addEventListener("click", () => {
          if (el.aiGlobalModelInput) {
            el.aiGlobalModelInput.value = pill.dataset.model;
            el.aiGlobalModelInput.dataset.userEdited = "true";
          }
        });
      });
    }

    // Render Persona Style selector (Formal vs Gen-Z)
    if (el.aiStyleGrid) {
      const activeStyle = state.aiConfig?.activeStyle || "formal";
      const styles = [
        { id: "formal", name: "Formal", sub: "Articulate, polite & professional (Default)" },
        { id: "genz", name: "Gen-Z", sub: "Savage roasts & reel brainrot" },
        { id: "self", name: "Self", sub: "Mimics your personal texting style per chat" }
      ];
      el.aiStyleGrid.innerHTML = styles.map(s => {
        const isCurrent = s.id === activeStyle;
        const border = isCurrent ? "var(--accent)" : "var(--border-default)";
        const bg = isCurrent ? "var(--accent-subtle)" : "var(--bg-subtle)";
        const badge = isCurrent ? `<span style="font-size: 10px; color: var(--accent); font-weight: 600;">● Active</span>` : '';
        return `
          <div class="style-card" data-style="${s.id}" style="padding: 10px; border: 2px solid ${border}; background: ${bg}; border-radius: var(--radius-sm); cursor: pointer; transition: all 0.15s ease;">
            <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 2px;">
              <span style="font-weight: 600; font-size: 13px; color: var(--text-primary);">${s.name}</span>
              ${badge}
            </div>
            <div style="font-size: 11px; color: var(--text-secondary);">${s.sub}</div>
          </div>
        `;
      }).join("");

      el.aiStyleGrid.querySelectorAll(".style-card").forEach(card => {
        card.addEventListener("click", async () => {
          const style = card.dataset.style;
          await setGlobalStyle(style);
        });
      });
    }

    // Populate chat select in overrides card
    if (el.aiOverrideChatSelect && state.chats && state.chats.length > 0) {
      const currentSelected = el.aiOverrideChatSelect.value;
      const options = ['<option value="">Select a chat...</option>'];
      state.chats.forEach(c => {
        const name = c.name || c.chatName || c.jid;
        options.push(`<option value="${escapeHtml(c.jid)}" ${c.jid === currentSelected ? 'selected' : ''}>${escapeHtml(name)} (${c.isGroup ? 'Group' : 'DM'})</option>`);
      });
      el.aiOverrideChatSelect.innerHTML = options.join("");
    }
  }

  function renderAIChatOverrides() {
    if (!el.aiChatOverridesContainer || !state.aiConfig) return;
    const overrides = state.aiConfig.chatOverrides || [];

    if (overrides.length === 0) {
      el.aiChatOverridesContainer.innerHTML = `
        <div style="font-size: 12px; color: var(--text-secondary); padding: 8px; text-align: center; border: 1px dashed var(--border-default); border-radius: var(--radius-sm);">
          No chat overrides configured. All chats use the global default (${(state.aiConfig.activeProvider || "gemini").toUpperCase()}).
        </div>
      `;
      return;
    }

    el.aiChatOverridesContainer.innerHTML = overrides.map(item => {
      const modelDisplay = item.model ? `<code style="font-size: 10px;">${escapeHtml(item.model.split('/').pop() || item.model)}</code>` : '<span style="color:var(--text-secondary); font-size: 11px;">Default model</span>';
      const styleDisplay = item.style === "genz"
        ? `<span class="badge" style="font-size: 10px; padding: 1px 4px; background: rgba(236,72,153,0.15); color: #ec4899; border: 1px solid rgba(236,72,153,0.3);">Gen-Z</span>`
        : (item.style === "self"
          ? `<span class="badge" style="font-size: 10px; padding: 1px 4px; background: rgba(16,185,129,0.15); color: #10b981; border: 1px solid rgba(16,185,129,0.3);">Self</span>`
          : (item.style === "formal"
            ? `<span class="badge" style="font-size: 10px; padding: 1px 4px; background: rgba(59,130,246,0.15); color: #3b82f6; border: 1px solid rgba(59,130,246,0.3);">Formal</span>`
            : ''));
      return `
        <div style="display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; background: var(--bg-surface); border: 1px solid var(--border-default); border-radius: var(--radius-sm); font-size: 12px;">
          <div style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-right: 8px;">
            <div style="font-weight: 600; color: var(--text-primary);">${escapeHtml(item.chatName || item.chatJid)}</div>
            <div style="font-size: 11px; color: var(--text-secondary); display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
              <span class="badge badge-primary" style="font-size: 10px; padding: 1px 4px;">${escapeHtml(item.provider.toUpperCase())}</span>
              ${styleDisplay}
              ${modelDisplay}
            </div>
          </div>
          <button class="btn btn-secondary btn-sm btn-delete-override" data-jid="${escapeHtml(item.chatJid)}" style="font-size: 11px; padding: 2px 8px;" title="Reset to global default">Reset</button>
        </div>
      `;
    }).join("");

    el.aiChatOverridesContainer.querySelectorAll(".btn-delete-override").forEach(btn => {
      btn.addEventListener("click", async () => {
        const jid = btn.dataset.jid;
        await deleteChatOverride(jid);
      });
    });
  }

  async function setGlobalProvider(provider) {
    try {
      const res = await fetch("/api/ai/provider", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ provider })
      });
      const data = await res.json();
      if (data.success) {
        showToast(`AI Provider switched to ${provider.toUpperCase()}`, "success");
        if (el.aiGlobalModelInput) {
          delete el.aiGlobalModelInput.dataset.userEdited;
        }
        await fetchAIConfig();
      } else {
        showToast(data.error || "Failed to switch provider", "error");
      }
    } catch (e) {
      showToast(e.message, "error");
    }
  }

  async function saveGlobalModel() {
    const model = el.aiGlobalModelInput?.value?.trim();
    if (!model) {
      showToast("Please enter a model name", "warning");
      return;
    }
    try {
      const res = await fetch("/api/ai/model", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model })
      });
      const data = await res.json();
      if (data.success) {
        showToast("Active model updated", "success");
        delete el.aiGlobalModelInput.dataset.userEdited;
        await fetchAIConfig();
      } else {
        showToast(data.error || "Failed to update model", "error");
      }
    } catch (e) {
      showToast(e.message, "error");
    }
  }

  async function setGlobalStyle(style) {
    try {
      const res = await fetch("/api/ai/style", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ style })
      });
      const data = await res.json();
      if (data.success) {
        showToast(`Default Persona Style set to ${style.toUpperCase()}`, "success");
        await fetchAIConfig();
      } else {
        showToast(data.error || "Failed to set style", "error");
      }
    } catch (e) {
      showToast(e.message, "error");
    }
  }

  async function saveChatOverride() {
    const chatJid = el.aiOverrideChatSelect?.value;
    const provider = el.aiOverrideProviderSelect?.value;
    const model = el.aiOverrideModelInput?.value?.trim();
    const style = el.aiOverrideStyleSelect?.value;

    if (!chatJid) {
      showToast("Please select a target chat", "warning");
      return;
    }

    try {
      const res = await fetch("/api/ai/chat-override", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ chatJid, provider, model, style })
      });
      const data = await res.json();
      if (data.success) {
        showToast(`Override saved for chat`, "success");
        if (el.aiOverrideModelInput) el.aiOverrideModelInput.value = "";
        await fetchAIConfig();
      } else {
        showToast(data.error || "Failed to save override", "error");
      }
    } catch (e) {
      showToast(e.message, "error");
    }
  }

  async function deleteChatOverride(chatJid) {
    try {
      const res = await fetch(`/api/ai/chat-override?chatJid=${encodeURIComponent(chatJid)}`, {
        method: "DELETE"
      });
      const data = await res.json();
      if (data.success) {
        showToast("Chat override removed. Now using global default.", "info");
        await fetchAIConfig();
      }
    } catch (e) {
      showToast(e.message, "error");
    }
  }

  async function fetchPersonas() {
    if (!el.personasListContainer) return;
    try {
      const res = await fetch("/api/personas");
      const data = await res.json();
      state.personas = data.personas || {};
      renderPersonasList();
    } catch (e) { }
  }

  function renderPersonasList() {
    if (!el.personasListContainer) return;
    const keys = Object.keys(state.personas);

    if (keys.length === 0) {
      el.personasListContainer.innerHTML = `<div class="empty-state"><div class="empty-title">No Personas Loaded</div></div>`;
      return;
    }

    el.personasListContainer.innerHTML = keys
      .map((k) => {
        const p = state.personas[k];
        const name = escapeHtml(p.name || p.label || k);
        const prefix = escapeHtml(p.prefix || `!${k}`);
        const prompt = escapeHtml(p.prompt || p.flavor || "");

        return `
          <div style="padding: 10px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface);">
            <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px;">
              <span style="font-weight: 600; font-size: 13px;">${name}</span>
              <code style="font-size: 11px; background: var(--bg-subtle); padding: 2px 6px; border-radius: var(--radius-sm);">${prefix}</code>
            </div>
            <div style="font-size: 11px; color: var(--text-secondary); line-height: 1.4;">${prompt}</div>
          </div>
        `;
      })
      .join("");

    // Populate playground dropdown
    if (el.playgroundPersonaSelect) {
      el.playgroundPersonaSelect.innerHTML = keys
        .map((k) => `<option value="${escapeHtml(k)}">${escapeHtml(state.personas[k].name || k)}</option>`)
        .join("");
    }
  }

  async function testPersona() {
    const personaKey = el.playgroundPersonaSelect?.value;
    const prompt = el.playgroundPromptInput?.value?.trim();

    if (!prompt) {
      showToast("Please enter a test prompt", "warning");
      return;
    }

    if (el.playgroundOutput) {
      el.playgroundOutput.textContent = "Generating dialogue response from Gemini...";
    }

    try {
      const res = await fetch("/api/personas/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ personaKey, prompt }),
      });
      const data = await res.json();
      if (data.success && el.playgroundOutput) {
        el.playgroundOutput.textContent = data.answer || "No response received.";
      } else {
        if (el.playgroundOutput) {
          el.playgroundOutput.textContent = `Error: ${data.error || "Failed to generate reply"}`;
        }
      }
    } catch (e) {
      if (el.playgroundOutput) {
        el.playgroundOutput.textContent = `Network error: ${e.message}`;
      }
    }
  }

  // ==========================================================================
  // Daily Status Reports & Group Deadlines
  // ==========================================================================
  async function fetchReportsData() {
    // Set default date if unset
    if (el.reportDatePicker && !el.reportDatePicker.value) {
      el.reportDatePicker.value = new Date().toISOString().split("T")[0];
    }
    const targetDate = el.reportDatePicker?.value || new Date().toISOString().split("T")[0];

    await Promise.all([
      fetchDailyReports(targetDate),
      fetchReportLeaderboard(),
      fetchReportGroups(),
      fetchDeadlines(),
    ]);
    updatePFPPreview();
  }

  async function fetchDailyReports(dateStr) {
    if (!el.reportsTableBody) return;
    try {
      const res = await fetch(`/api/reports?date=${encodeURIComponent(dateStr)}`);
      const data = await res.json();
      const reports = data.reports || [];

      if (el.reportsCountBadge) {
        el.reportsCountBadge.textContent = reports.length;
      }

      if (reports.length === 0) {
        el.reportsTableBody.innerHTML = `<tr><td colspan="3" style="text-align: center; color: var(--text-muted); padding: 16px;">No achievements submitted yet for ${escapeHtml(dateStr)}.</td></tr>`;
        return;
      }

      el.reportsTableBody.innerHTML = reports
        .map((r) => {
          const time = r.timestamp ? new Date(r.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";
          const name = escapeHtml(r.senderName || r.userJid);
          const content = escapeHtml(r.content);
          return `
            <tr>
              <td style="font-weight: 600; white-space: nowrap;">${name}</td>
              <td style="font-size: 11px; color: var(--text-muted); white-space: nowrap;">${time}</td>
              <td style="font-size: 12px; line-height: 1.4;">${content}</td>
            </tr>
          `;
        })
        .join("");
    } catch (e) {
      el.reportsTableBody.innerHTML = `<tr><td colspan="3" style="text-align: center; color: var(--color-danger);">Failed to load reports: ${escapeHtml(e.message)}</td></tr>`;
    }
  }

  async function generateAISummary(dateStr) {
    if (!el.reportSummaryContent) return;
    el.reportSummaryContent.textContent = "Synthesizing Daily Status Report with Gemini AI...";
    if (el.reportSummaryStatusBadge) {
      el.reportSummaryStatusBadge.textContent = "Synthesizing...";
      el.reportSummaryStatusBadge.className = "badge badge-accent";
    }

    try {
      const res = await fetch("/api/reports/generate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ date: dateStr }),
      });
      const data = await res.json();
      if (data.summary) {
        el.reportSummaryContent.textContent = data.summary;
        if (el.reportSummaryStatusBadge) {
          el.reportSummaryStatusBadge.textContent = "Generated";
          el.reportSummaryStatusBadge.className = "badge badge-success";
        }
        showToast("AI Summary generated successfully!", "success");
      } else {
        el.reportSummaryContent.textContent = `Failed: ${data.error || "Unknown error"}`;
        if (el.reportSummaryStatusBadge) {
          el.reportSummaryStatusBadge.textContent = "Error";
          el.reportSummaryStatusBadge.className = "badge badge-danger";
        }
      }
    } catch (e) {
      el.reportSummaryContent.textContent = `Network error: ${e.message}`;
    }
  }

  async function fetchReportLeaderboard() {
    if (!el.leaderboardMiniBody) return;
    try {
      const res = await fetch("/api/reports/leaderboard");
      const list = await res.json();

      if (!list || list.length === 0) {
        el.leaderboardMiniBody.innerHTML = `<tr><td colspan="4" style="text-align: center; color: var(--text-muted); padding: 12px;">No participants yet.</td></tr>`;
        return;
      }

      el.leaderboardMiniBody.innerHTML = list
        .slice(0, 8)
        .map((u, i) => {
          let rank = `#${i + 1}`;
          const name = escapeHtml(u.name || u.userJid.split("@")[0]);
          return `
            <tr>
              <td style="font-weight: 700; width: 28px;">${rank}</td>
              <td style="font-weight: 500;">${name}</td>
              <td style="font-weight: 600; color: var(--accent-primary);">${u.points}</td>
              <td><span class="badge badge-accent" style="font-size: 10px;"><svg class="icon-inline" style="width: 10px; height: 10px;"><use href="#icon-zap"/></svg> ${u.currentStreak}d</span></td>
            </tr>
          `;
        })
        .join("");
    } catch (e) {
      el.leaderboardMiniBody.innerHTML = `<tr><td colspan="4" style="text-align: center; color: var(--color-danger);">Error loading leaderboard</td></tr>`;
    }
  }

  async function fetchReportGroups() {
    if (!el.reportGroupsList) return;
    try {
      const res = await fetch("/api/reports/groups");
      const data = await res.json();
      const groups = data.groups || [];

      if (groups.length === 0) {
        el.reportGroupsList.innerHTML = `<span style="font-size: 11px; color: var(--text-muted);">No target broadcast groups configured yet.</span>`;
        return;
      }

      el.reportGroupsList.innerHTML = groups
        .map((g) => {
          const clean = escapeHtml(g);
          return `
            <div style="display: flex; align-items: center; justify-content: space-between; padding: 4px 8px; background: var(--bg-surface); border: 1px solid var(--border-muted); border-radius: var(--radius-sm); font-size: 11px;">
              <span style="font-family: var(--font-mono); overflow: hidden; text-overflow: ellipsis; max-width: 170px;">${clean}</span>
              <button class="btn btn-sm btn-danger btn-remove-report-group" data-jid="${clean}" style="padding: 2px 6px; font-size: 10px;">✕</button>
            </div>
          `;
        })
        .join("");
    } catch (e) {
      el.reportGroupsList.innerHTML = `<span style="color: var(--color-danger); font-size: 11px;">Error loading groups</span>`;
    }
  }

  async function addReportGroup(jid) {
    if (!jid) return;
    try {
      const res = await fetch("/api/reports/groups", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ groupJid: jid }),
      });
      const data = await res.json();
      if (data.success) {
        showToast("Added target report group", "success");
        if (el.newReportGroupInput) el.newReportGroupInput.value = "";
        fetchReportGroups();
      } else {
        showToast(data.error || "Failed to add group", "danger");
      }
    } catch (e) {
      showToast(`Network error: ${e.message}`, "danger");
    }
  }

  async function removeReportGroup(jid) {
    if (!jid) return;
    try {
      const res = await fetch(`/api/reports/groups?groupJid=${encodeURIComponent(jid)}`, {
        method: "DELETE",
      });
      const data = await res.json();
      if (data.success) {
        showToast("Removed target report group", "success");
        fetchReportGroups();
      }
    } catch (e) {
      showToast(`Network error: ${e.message}`, "danger");
    }
  }

  async function fetchDeadlines() {
    if (!el.deadlinesTableBody) return;
    try {
      const res = await fetch("/api/deadlines");
      const list = await res.json();

      if (!list || list.length === 0) {
        el.deadlinesTableBody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 16px;">No group deadline countdowns active. Set one above!</td></tr>`;
        return;
      }

      const now = new Date();
      now.setHours(0, 0, 0, 0);

      el.deadlinesTableBody.innerHTML = list
        .map((d) => {
          const target = new Date(d.targetDate);
          const diffDays = Math.ceil((target - now) / (1000 * 60 * 60 * 24));
          let statusBadge = `<span class="badge badge-accent">${diffDays} days left</span>`;
          if (diffDays === 1) {
            statusBadge = `<span class="badge badge-warning">1 day left</span>`;
          } else if (diffDays === 0) {
            statusBadge = `<span class="badge badge-danger">D-Day Today</span>`;
          } else if (diffDays < 0) {
            statusBadge = `<span class="badge badge-neutral">Completed</span>`;
          }

          const jid = escapeHtml(d.groupJid);
          const title = escapeHtml(d.title);
          const date = escapeHtml(d.targetDate);

          return `
            <tr>
              <td style="font-family: var(--font-mono); font-size: 11px;">${jid}</td>
              <td style="font-weight: 600;">${title}</td>
              <td style="font-size: 12px;">${date}</td>
              <td>${statusBadge}</td>
              <td style="display: flex; gap: 6px;">
                <button class="btn btn-sm btn-primary btn-update-group-pfp" data-jid="${jid}" title="Trigger instant WhatsApp PFP update">Update PFP</button>
                <button class="btn btn-sm btn-danger btn-delete-deadline" data-jid="${jid}" title="Remove countdown">Remove</button>
              </td>
            </tr>
          `;
        })
        .join("");
    } catch (e) {
      el.deadlinesTableBody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--color-danger);">Failed to load deadlines: ${escapeHtml(e.message)}</td></tr>`;
    }
  }

  function updatePFPPreview() {
    if (!el.pfpLivePreviewImg) return;
    const title = el.pfpTitleInput?.value?.trim() || "Goal Deadline";
    const date = el.pfpDateInput?.value || new Date(Date.now() + 14 * 86400000).toISOString().split("T")[0];
    el.pfpLivePreviewImg.src = `/api/deadlines/preview?title=${encodeURIComponent(title)}&date=${encodeURIComponent(date)}&_t=${Date.now()}`;
  }

  async function saveDeadline() {
    const groupJid = el.pfpGroupJidInput?.value?.trim();
    const targetDate = el.pfpDateInput?.value;
    const title = el.pfpTitleInput?.value?.trim() || "Goal Countdown";

    if (!groupJid || !targetDate) {
      showToast("Group JID and Target Date are required", "warning");
      return;
    }

    try {
      const res = await fetch("/api/deadlines", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          groupJid,
          targetDate,
          title,
          updateNow: true,
        }),
      });
      const data = await res.json();
      if (data.success) {
        showToast("Group countdown deadline saved and icon update triggered!", "success");
        fetchDeadlines();
      } else {
        showToast(data.error || "Failed to save deadline", "danger");
      }
    } catch (e) {
      showToast(`Network error: ${e.message}`, "danger");
    }
  }

  async function forceUpdateGroupPFP(jid) {
    if (!jid) return;
    try {
      showToast(`Updating WhatsApp group icon for ${jid}...`, "warning");
      const res = await fetch("/api/deadlines/updatepfp", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ groupJid: jid }),
      });
      const data = await res.json();
      if (data.success) {
        showToast("Group profile picture updated successfully on WhatsApp!", "success");
      } else {
        showToast(data.error || "WhatsApp PFP update failed", "danger");
      }
    } catch (e) {
      showToast(`Network error: ${e.message}`, "danger");
    }
  }

  async function removeDeadline(jid) {
    if (!jid) return;
    try {
      const res = await fetch(`/api/deadlines?groupJid=${encodeURIComponent(jid)}`, {
        method: "DELETE",
      });
      const data = await res.json();
      if (data.success) {
        showToast("Group deadline countdown removed", "success");
        fetchDeadlines();
      }
    } catch (e) {
      showToast(`Network error: ${e.message}`, "danger");
    }
  }

  // 6. Auto-Reply
  async function fetchAutoReply() {
    try {
      const res = await fetch("/api/autoreply");
      const data = await res.json();
      if (el.badgeAutoReplyState) {
        el.badgeAutoReplyState.textContent = data.enabled ? "State: Active" : "State: Disabled";
        el.badgeAutoReplyState.className = `badge ${data.enabled ? "badge-success" : "badge-neutral"}`;
      }
      if (el.selectAutoReplyMode) el.selectAutoReplyMode.value = data.mode || "keyword";

      // Render triggers table
      if (el.triggersTableBody) {
        const triggers = data.triggers || {};
        const keys = Object.keys(triggers);
        if (keys.length === 0) {
          el.triggersTableBody.innerHTML = `<tr><td colspan="3" style="text-align: center; padding: 16px; color: var(--text-muted);">No keyword triggers configured.</td></tr>`;
        } else {
          el.triggersTableBody.innerHTML = keys
            .map((k) => `
              <tr>
                <td><code>${escapeHtml(k)}</code></td>
                <td>${escapeHtml(triggers[k])}</td>
                <td><span class="badge badge-success">Active</span></td>
              </tr>
            `)
            .join("");
        }
      }
    } catch (e) { }
  }

  // 7. Reminders
  async function fetchReminders() {
    if (!el.remindersTableBody) return;
    try {
      const res = await fetch("/api/reminders");
      const list = await res.json();
      renderRemindersTable(list);
    } catch (e) {
      if (el.remindersTableBody) {
        el.remindersTableBody.innerHTML = `<tr><td colspan="5" style="text-align: center; padding: 18px;">Failed to load reminders.</td></tr>`;
      }
    }
  }

  function renderRemindersTable(list) {
    if (!el.remindersTableBody) return;
    if (!list || list.length === 0) {
      el.remindersTableBody.innerHTML = `
        <tr>
          <td colspan="5">
            <div class="empty-state">
              <svg><use href="#icon-clock"/></svg>
              <div class="empty-title">No Active Reminders</div>
              <div class="empty-desc">Schedule one using the button above or via chat command !remind.</div>
            </div>
          </td>
        </tr>
      `;
      return;
    }

    el.remindersTableBody.innerHTML = list
      .map((item) => {
        const due = item.targetTime
          ? new Date(item.targetTime).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
          : "Soon";
        const id = escapeHtml(item.id || "");
        const chat = escapeHtml(item.chatName || item.jid || "Direct Message");
        const text = escapeHtml(item.text || "");

        return `
          <tr>
            <td style="font-family: var(--font-mono); font-size: 11px;">${id}</td>
            <td>${chat}</td>
            <td><span class="badge badge-warning">${due}</span></td>
            <td>${text}</td>
            <td>
              <button class="btn btn-sm btn-danger btn-cancel-remind" data-id="${id}">Cancel</button>
            </td>
          </tr>
        `;
      })
      .join("");

    el.remindersTableBody.querySelectorAll(".btn-cancel-remind").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const id = btn.getAttribute("data-id");
        try {
          const res = await fetch(`/api/reminders/${encodeURIComponent(id)}`, { method: "DELETE" });
          if (res.ok) {
            showToast("Reminder cancelled", "success");
            fetchReminders();
          }
        } catch (e) {
          showToast("Failed to cancel reminder", "danger");
        }
      });
    });
  }

  // 8. Notes
  async function fetchNotes() {
    if (!el.notesTreeContainer) return;
    try {
      const res = await fetch("/api/notes");
      const data = await res.json();
      renderNotesTree(data);
    } catch (e) {
      if (el.notesTreeContainer) {
        el.notesTreeContainer.innerHTML = `<div style="color: var(--danger-text);">Failed to load notes tree.</div>`;
      }
    }
  }

  function renderNotesTree(tree) {
    if (!el.notesTreeContainer) return;
    if (!tree || Object.keys(tree).length === 0) {
      el.notesTreeContainer.innerHTML = `
        <div class="empty-state">
          <svg><use href="#icon-folder"/></svg>
          <div class="empty-title">Notes Vault Empty</div>
          <div class="empty-desc">Create notes via chat command !addnote &lt;path&gt; &lt;text&gt;.</div>
        </div>
      `;
      return;
    }

    let html = `<div style="color: var(--accent-text); font-weight: 700; margin-bottom: 8px;">root/</div>`;
    function renderBranch(node, depth = 1) {
      const indent = "&nbsp;&nbsp;".repeat(depth);
      for (const name in node) {
        const item = node[name];
        if (typeof item === "object" && item !== null && !item.content && !item.file) {
          html += `<div>${indent}<svg class="icon-inline" style="color: var(--accent);"><use href="#icon-folder"/></svg> <strong style="color: var(--text-primary);">${escapeHtml(name)}/</strong></div>`;
          renderBranch(item, depth + 1);
        } else {
          const content = typeof item === "string" ? item : (item.content || "");
          const preview = content.substring(0, 45).replace(/\n/g, " ");
          html += `<div>${indent}<svg class="icon-inline" style="color: var(--text-muted);"><use href="#icon-file"/></svg> <span style="color: var(--text-primary);">${escapeHtml(name)}</span> <span style="color: var(--text-muted);">"${escapeHtml(preview)}"</span></div>`;
        }
      }
    }
    renderBranch(tree);
    el.notesTreeContainer.innerHTML = html;
  }

  // 9. Storage
  async function fetchStorage() {
    try {
      const res = await fetch("/api/storage");
      const data = await res.json();
      if (el.storageMediaSize) el.storageMediaSize.textContent = `${data.mediaMB || 0} MB`;
      if (el.storageDbSize) el.storageDbSize.textContent = `${data.dbMB || 0} MB`;
      if (el.storageNotesSize) el.storageNotesSize.textContent = `${data.notesKB || 0} KB`;
      if (el.storageAuditSize) el.storageAuditSize.textContent = `${data.deletedKB || 0} KB`;
    } catch (e) { }
  }

  async function cleanMediaStorage() {
    try {
      const res = await fetch("/api/storage/clean", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ target: "media" }),
      });
      const data = await res.json();
      showToast(`Cleaned ${data.freedMB || 0} MB temporary media`, "success");
      fetchStorage();
    } catch (e) {
      showToast("Failed to clean media storage", "danger");
    }
  }

  // 10. Console Logs
  async function fetchLogs() {
    if (!el.logTerminal) return;
    try {
      const res = await fetch("/api/logs");
      const logs = await res.json();
      state.logs = Array.isArray(logs) ? logs : [];
      renderLogs();
    } catch (e) { }
  }

  function renderLogs() {
    if (!el.logTerminal) return;
    const filter = state.logsFilter;

    const filtered = state.logs.filter((entry) => {
      if (filter === "ALL") return true;
      return (entry.level || "").toUpperCase() === filter;
    });

    if (filtered.length === 0) {
      el.logTerminal.innerHTML = `<div style="color: var(--text-muted); text-align: center; padding: 20px;">No logs recorded yet.</div>`;
      return;
    }

    el.logTerminal.innerHTML = filtered
      .map((entry) => {
        const time = entry.timestamp
          ? new Date(entry.timestamp).toLocaleTimeString([], { hour12: false })
          : "--:--:--";
        const lvl = (entry.level || "INFO").toUpperCase();
        return `
          <div class="log-line">
            <span class="log-time">[${time}]</span>
            <span class="log-level ${lvl}">${lvl}</span>
            <span>${escapeHtml(entry.message || "")}</span>
          </div>
        `;
      })
      .join("");

    if (state.autoScrollLogs) {
      el.logTerminal.scrollTop = el.logTerminal.scrollHeight;
    }
  }

  // MeowCaller VoIP & Song Party Management
  async function fetchMeowCallerStatus() {
    try {
      const res = await fetch("/api/meowcaller/status");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      if (data && data.status) {
        updateMeowCallerStatusUI(data.status);
      }
    } catch (err) {
      console.error("Failed to fetch MeowCaller status:", err);
    }
  }

  function updateMeowCallerStatusUI(status) {
    if (!status) return;
    state.meowcaller = status;

    const isEnabled = !!status.enabled;

    // Overview Badge & Label
    if (el.badgeMeowCallerState) {
      el.badgeMeowCallerState.textContent = isEnabled ? "LIVE" : "OFF";
      el.badgeMeowCallerState.className = `badge ${isEnabled ? "badge-success" : "badge-neutral"}`;
    }
    if (el.labelMeowCallerOverview) {
      if (!isEnabled) {
        el.labelMeowCallerOverview.textContent = "Off by default (Safe mode)";
      } else if (status.tempSecondsLeft > 0) {
        const m = Math.floor(status.tempSecondsLeft / 60);
        el.labelMeowCallerOverview.textContent = `Active (${m}m window remaining)`;
      } else {
        el.labelMeowCallerOverview.textContent = "Active (Permanent mode)";
      }
    }

    // Sidebar Badge
    if (el.sidebarCallerBadge) {
      if (!isEnabled) {
        el.sidebarCallerBadge.textContent = "OFF";
        el.sidebarCallerBadge.className = "badge badge-neutral";
      } else if (status.tempSecondsLeft > 0) {
        const m = Math.floor(status.tempSecondsLeft / 60);
        el.sidebarCallerBadge.textContent = `${m}m`;
        el.sidebarCallerBadge.className = "badge badge-warning";
      } else {
        el.sidebarCallerBadge.textContent = "LIVE";
        el.sidebarCallerBadge.className = "badge badge-success";
      }
    }

    // Master Status Badge in MeowCaller tab
    if (el.callerStatusBadge) {
      el.callerStatusBadge.innerHTML = isEnabled ? '<span class="status-dot online"></span> LIVE (Active VoIP)' : 'OFF (Safe Mode)';
      el.callerStatusBadge.className = `badge ${isEnabled ? "badge-success" : "badge-neutral"}`;
    }

    // Remaining Timer Badge
    if (el.callerTimerRemainingBadge) {
      if (isEnabled && status.tempSecondsLeft > 0) {
        const m = Math.floor(status.tempSecondsLeft / 60);
        const s = status.tempSecondsLeft % 60;
        el.callerTimerRemainingBadge.style.display = "inline-flex";
        el.callerTimerRemainingBadge.innerHTML = `<svg class="icon-inline"><use href="#icon-clock"/></svg> ${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")} remaining`;
      } else {
        el.callerTimerRemainingBadge.style.display = "none";
      }
    }

    // Master Toggle Button
    if (el.btnToggleCallerMaster) {
      if (isEnabled) {
        el.btnToggleCallerMaster.className = "btn btn-danger";
        if (el.callerMasterBtnIcon) el.callerMasterBtnIcon.innerHTML = '<svg class="icon-inline"><use href="#icon-square"/></svg>';
        if (el.callerMasterBtnText) el.callerMasterBtnText.textContent = "Disable MeowCaller";
      } else {
        el.btnToggleCallerMaster.className = "btn btn-primary";
        if (el.callerMasterBtnIcon) el.callerMasterBtnIcon.innerHTML = '<svg class="icon-inline"><use href="#icon-zap"/></svg>';
        if (el.callerMasterBtnText) el.callerMasterBtnText.textContent = "Enable MeowCaller";
      }
    }

    // Auto-Accept Checkbox
    if (el.checkCallerAutoAccept) {
      el.checkCallerAutoAccept.checked = !!status.autoAcceptCalls;
    }

    // Pending Incoming Call Alert
    if (el.callerIncomingAlert) {
      if (status.pendingCall) {
        el.callerIncomingAlert.style.display = "block";
        if (el.callerIncomingPeer) {
          el.callerIncomingPeer.textContent = status.pendingCall.peer || "Unknown Caller";
        }
        if (el.callerIncomingDuration) {
          el.callerIncomingDuration.textContent = formatTime(status.pendingCall.durationSec || 0);
        }
      } else {
        el.callerIncomingAlert.style.display = "none";
      }
    }

    // Call Console
    const call = status.activeCall;
    if (el.callerCallPhaseBadge) {
      const phase = call ? call.phase : (status.pendingCall ? "Ringing" : "Idle");
      el.callerCallPhaseBadge.textContent = capitalize(phase);
      el.callerCallPhaseBadge.className = `badge ${call && phase === "active" ? "badge-success" : (call || status.pendingCall ? "badge-warning" : "badge-neutral")}`;
    }
    if (el.callerCallDuration) {
      el.callerCallDuration.textContent = call && call.durationSec ? formatTime(call.durationSec) : "00:00";
    }
    if (el.callerCallPeer) {
      el.callerCallPeer.textContent = call ? call.peer : (status.pendingCall ? `Incoming: ${status.pendingCall.peer}` : "No Call in Progress");
    }
    if (el.callerCallSubtext) {
      if (call) {
        el.callerCallSubtext.textContent = call.isOutgoing ? "Outgoing WhatsApp VoIP Call" : "Incoming WhatsApp VoIP Call";
      } else if (status.pendingCall) {
        el.callerCallSubtext.textContent = "Incoming Call Ringing... Click 'Answer Call' above to accept";
      } else {
        el.callerCallSubtext.textContent = "Pure-Go VoIP Stack (MLOW Codec)";
      }
    }
    if (el.callerAvatarIcon) {
      el.callerAvatarIcon.innerHTML = call
        ? (call.phase === "active" ? '<svg class="icon-inline" style="width: 18px; height: 18px;"><use href="#icon-volume"/></svg>' : '<svg class="icon-inline" style="width: 18px; height: 18px;"><use href="#icon-phone"/></svg>')
        : (status.pendingCall ? '<svg class="icon-inline" style="width: 18px; height: 18px;"><use href="#icon-phone"/></svg>' : '<svg class="icon-inline" style="width: 18px; height: 18px;"><use href="#icon-phone"/></svg>');
    }

    // Live call audio monitor notice and auto-listening synchronization
    if (call && call.phase === "active") {
      if (el.callerLiveAudioNotice) el.callerLiveAudioNotice.style.display = "flex";
      if (!isListening) {
        startAudioMonitoring();
      }
      if (audioContext && audioContext.state === "suspended") {
        if (el.callerLiveAudioNoticeText) {
          el.callerLiveAudioNoticeText.textContent = "Live call audio is stream ready. Click 'Unmute Audio' to hear caller.";
        }
        if (el.btnQuickStartListening) {
          el.btnQuickStartListening.textContent = "Unmute Audio";
          el.btnQuickStartListening.className = "btn btn-sm btn-warning";
        }
      } else {
        if (el.callerLiveAudioNoticeText) {
          el.callerLiveAudioNoticeText.textContent = "Live caller audio is streaming to your speakers.";
        }
        if (el.btnQuickStartListening) {
          el.btnQuickStartListening.textContent = "Listening Live";
          el.btnQuickStartListening.className = "btn btn-sm btn-success";
        }
      }
    } else {
      if (el.callerLiveAudioNotice) el.callerLiveAudioNotice.style.display = "none";
    }

    // Song Party DJ Deck
    const isPlaying = !!status.isPlaying;
    const isPaused = !!status.isPaused;
    const curr = status.currentTrack;

    if (el.partyVisualizer) {
      if (isPlaying && !isPaused) {
        el.partyVisualizer.classList.add("playing");
      } else {
        el.partyVisualizer.classList.remove("playing");
      }
    }

    if (el.partyPlaybackBadge) {
      if (isPaused) {
        el.partyPlaybackBadge.textContent = "PAUSED";
        el.partyPlaybackBadge.className = "badge badge-warning";
      } else if (isPlaying) {
        el.partyPlaybackBadge.textContent = "STREAMING";
        el.partyPlaybackBadge.className = "badge badge-success";
      } else {
        el.partyPlaybackBadge.textContent = "IDLE";
        el.partyPlaybackBadge.className = "badge badge-neutral";
      }
    }

    if (el.partyNowPlayingTitle) {
      el.partyNowPlayingTitle.textContent = curr ? curr.title : "No Track Playing";
    }
    if (el.partyNowPlayingRequester) {
      if (curr) {
        el.partyNowPlayingRequester.textContent = `Requested by ${curr.requestedBy || "DJ"} • Streaming in call`;
      } else {
        el.partyNowPlayingRequester.innerHTML = `Add a song below or send <code>!party start &lt;song&gt;</code> in WhatsApp`;
      }
    }

    if (el.partyPauseResumeIcon && el.partyPauseResumeText) {
      if (isPaused) {
        el.partyPauseResumeIcon.innerHTML = '<svg class="icon-inline"><use href="#icon-play"/></svg>';
        el.partyPauseResumeText.textContent = "Resume";
      } else {
        el.partyPauseResumeIcon.innerHTML = '<svg class="icon-inline"><use href="#icon-pause"/></svg>';
        el.partyPauseResumeText.textContent = "Pause";
      }
    }

    // Upcoming Queue Table
    const queue = status.queue || [];
    if (el.partyQueueCountBadge) {
      el.partyQueueCountBadge.textContent = `${queue.length} song${queue.length === 1 ? "" : "s"}`;
    }

    if (el.partyQueueTableBody) {
      if (queue.length === 0) {
        el.partyQueueTableBody.innerHTML = `
          <tr>
            <td colspan="5" style="text-align: center; color: var(--text-muted); padding: 24px;">
              Queue is empty. Search for songs above or use <code>!party add &lt;song&gt;</code> on WhatsApp.
            </td>
          </tr>
        `;
      } else {
        el.partyQueueTableBody.innerHTML = queue.map((t, idx) => `
          <tr>
            <td style="font-family: var(--font-mono); color: var(--text-muted);">${idx + 1}</td>
            <td style="font-weight: 600; color: var(--text-primary);">${escapeHtml(t.title)}</td>
            <td style="color: var(--text-secondary);">${escapeHtml(t.requestedBy || "User")}</td>
            <td style="font-size: 11px; color: var(--text-muted);">${t.addedAt ? new Date(t.addedAt).toLocaleTimeString() : "-"}</td>
            <td style="text-align: right;">
              <button class="btn btn-sm btn-danger btn-remove-party-track" data-index="${idx}" title="Remove track" style="padding: 2px 6px;">
                <svg style="width: 11px; height: 11px;"><use href="#icon-trash"/></svg>
              </button>
            </td>
          </tr>
        `).join("");
      }
    }

    // Populate datalist with recent chats if empty
    if (el.callerRecentRecipients && el.callerRecentRecipients.children.length === 0 && state.chats && state.chats.length > 0) {
      el.callerRecentRecipients.innerHTML = state.chats.map(c => `
        <option value="${escapeHtml(c.jid)}">${escapeHtml(c.name || c.jid)}</option>
      `).join("");
    }

    // Realtime AI VoiceBot Telemetry & UI
    const vb = status.voiceBot;
    if (vb) {
      const vbEnabled = !!vb.enabled;
      if (el.btnToggleVoiceBot) {
        if (vbEnabled) {
          el.btnToggleVoiceBot.className = "btn btn-sm btn-danger";
          if (el.voiceBotBtnIcon) el.voiceBotBtnIcon.innerHTML = '<svg class="icon-inline"><use href="#icon-square"/></svg>';
          if (el.voiceBotBtnText) el.voiceBotBtnText.textContent = "Disable VoiceBot";
        } else {
          el.btnToggleVoiceBot.className = "btn btn-sm btn-primary";
          if (el.voiceBotBtnIcon) el.voiceBotBtnIcon.innerHTML = '<svg class="icon-inline"><use href="#icon-mic"/></svg>';
          if (el.voiceBotBtnText) el.voiceBotBtnText.textContent = "Enable VoiceBot";
        }
      }

      if (el.voiceBotLiveDot) {
        el.voiceBotLiveDot.className = vbEnabled ? "live-dot active" : "live-dot";
      }

      if (el.voiceBotStatusBadge) {
        if (!vbEnabled) {
          el.voiceBotStatusBadge.textContent = "STANDBY";
          el.voiceBotStatusBadge.className = "badge badge-neutral";
        } else {
          const stateUpper = (vb.state || "listening").toUpperCase();
          el.voiceBotStatusBadge.textContent = stateUpper;
          if (vb.state === "speaking") {
            el.voiceBotStatusBadge.className = "badge badge-success";
          } else if (vb.state === "thinking" || vb.state === "transcribing") {
            el.voiceBotStatusBadge.className = "badge badge-warning";
          } else if (vb.state === "offline") {
            el.voiceBotStatusBadge.className = "badge badge-danger";
          } else {
            el.voiceBotStatusBadge.className = "badge badge-primary";
          }
        }
      }

      if (el.voiceBotWsConnBadge) {
        if (vbEnabled) {
          el.voiceBotWsConnBadge.textContent = "Groq Whisper STT";
          el.voiceBotWsConnBadge.className = "badge badge-success";
        } else {
          el.voiceBotWsConnBadge.textContent = "Standby";
          el.voiceBotWsConnBadge.className = "badge badge-neutral";
        }
      }

      if (el.voiceBotLatencyBadge) {
        if (vb.lastLatencyMs > 0) {
          el.voiceBotLatencyBadge.style.display = "inline-flex";
          el.voiceBotLatencyBadge.textContent = `${vb.lastLatencyMs}ms Latency`;
        } else {
          el.voiceBotLatencyBadge.style.display = "none";
        }
      }

      if (el.voiceBotMetricLatency) {
        el.voiceBotMetricLatency.textContent = vb.lastLatencyMs > 0 ? `${vb.lastLatencyMs} ms` : "-- ms";
      }

      if (el.voiceBotMetricContext) {
        el.voiceBotMetricContext.textContent = `${vb.contextTurns || 0} turns`;
      }

      if (el.voiceBotMetricTurns) {
        el.voiceBotMetricTurns.textContent = String(vb.totalTurns || 0);
      }

      if (el.voiceBotLiveTranscript && vb.lastTranscript) {
        el.voiceBotLiveTranscript.textContent = vb.lastTranscript;
        el.voiceBotLiveTranscript.style.fontStyle = "normal";
      }

      if (el.voiceBotLiveResponse && vb.lastResponse) {
        el.voiceBotLiveResponse.textContent = vb.lastResponse;
      }

      if (el.voiceBotAsrStateBadge) {
        if (vb.state === "transcribing") {
          el.voiceBotAsrStateBadge.textContent = "Transcribing Speech...";
          el.voiceBotAsrStateBadge.style.color = "#38bdf8";
        } else if (vb.state === "listening") {
          el.voiceBotAsrStateBadge.textContent = "Listening (100ms chunks)";
          el.voiceBotAsrStateBadge.style.color = "#34d399";
        } else {
          el.voiceBotAsrStateBadge.textContent = "Standby";
          el.voiceBotAsrStateBadge.style.color = "var(--text-muted)";
        }
      }

      if (el.voiceBotTtsStateBadge) {
        if (vb.state === "speaking") {
          el.voiceBotTtsStateBadge.textContent = "Speaking into Call (Barge-in active)";
          el.voiceBotTtsStateBadge.style.color = "#34d399";
        } else {
          el.voiceBotTtsStateBadge.textContent = "Idle";
          el.voiceBotTtsStateBadge.style.color = "var(--text-muted)";
        }
      }

      if (el.voiceBotSongTriggerInput && vb.songTriggerPhrase && document.activeElement !== el.voiceBotSongTriggerInput) {
        el.voiceBotSongTriggerInput.value = vb.songTriggerPhrase;
      }
    }

    if (status.radio) {
      const radio = status.radio;
      if (el.radioSpeakingBadge) {
        el.radioSpeakingBadge.style.display = radio.isSpeaking ? "inline-flex" : "none";
      }
      if (el.radioMixFeedback) {
        if (radio.micActive) {
          el.radioMixFeedback.textContent = radio.mode === "ducking" ? "Ducking Active (Music @ 15%)" : "Music Muted";
        } else {
          el.radioMixFeedback.textContent = "";
        }
      }
    }
  }

  async function toggleMeowCallerMaster() {
    const isCurrentlyEnabled = state.meowcaller && state.meowcaller.enabled;
    const newEnabled = !isCurrentlyEnabled;
    const duration = newEnabled ? state.selectedDurationMinutes : 0;

    try {
      const res = await fetch("/api/meowcaller/toggle", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: newEnabled, durationMinutes: duration }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      if (data && data.status) {
        updateMeowCallerStatusUI(data.status);
        showToast(newEnabled ? `MeowCaller enabled (${duration > 0 ? duration + "m window" : "indefinitely"})` : "MeowCaller disabled", newEnabled ? "success" : "info");
      }
    } catch (err) {
      showToast(`Failed to toggle MeowCaller: ${err.message}`, "danger");
    }
  }

  async function toggleCallerAutoAccept(autoAccept) {
    try {
      const res = await fetch("/api/meowcaller/auto-accept", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ autoAccept }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      if (data && data.status) {
        updateMeowCallerStatusUI(data.status);
        showToast(`Auto-answer calls set to ${autoAccept ? "ON" : "OFF"}`);
      }
    } catch (err) {
      showToast(`Failed to update auto-answer: ${err.message}`, "danger");
    }
  }

  async function dialCaller() {
    const target = el.callerDialTarget?.value?.trim();
    if (!target) {
      showToast("Please enter a phone number or group JID to dial", "warning");
      return;
    }

    if (!state.meowcaller || !state.meowcaller.enabled) {
      showToast("MeowCaller is disabled. Please enable it first.", "warning");
      return;
    }

    showToast(`Dialing ${target}...`, "info");
    try {
      const res = await fetch("/api/meowcaller/call", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ target }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      showToast(`Call initiated to ${target}`, "success");
      fetchMeowCallerStatus();
      if (!isListening) {
        startAudioMonitoring();
      }
    } catch (err) {
      showToast(`Dial error: ${err.message}`, "danger");
    }
  }

  async function hangupCaller() {
    try {
      const res = await fetch("/api/meowcaller/hangup", { method: "POST" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      showToast("Call hung up", "info");
      fetchMeowCallerStatus();
    } catch (err) {
      showToast(`Hang up error: ${err.message}`, "danger");
    }
  }

  async function answerIncomingCaller() {
    try {
      const res = await fetch("/api/meowcaller/answer", { method: "POST" });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      showToast("Incoming call answered! Audio live.", "success");
      fetchMeowCallerStatus();
      if (!isListening) {
        startAudioMonitoring();
      }
    } catch (err) {
      showToast(`Answer error: ${err.message}`, "danger");
    }
  }

  async function rejectIncomingCaller() {
    try {
      const res = await fetch("/api/meowcaller/reject", { method: "POST" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      showToast("Incoming call rejected", "info");
      fetchMeowCallerStatus();
    } catch (err) {
      showToast(`Reject error: ${err.message}`, "danger");
    }
  }

  async function queuePartySongFromDashboard() {
    const query = el.partySongInput?.value?.trim();
    if (!query) {
      showToast("Please enter a song name or YouTube link", "warning");
      return;
    }

    if (!state.meowcaller || !state.meowcaller.enabled) {
      showToast("MeowCaller is disabled. Enable it before starting the song party.", "warning");
      return;
    }

    const target = el.callerDialTarget?.value?.trim() || "";
    showToast(`Downloading & preparing "${query}"...`, "info");
    if (el.btnPartyQueueSong) {
      el.btnPartyQueueSong.disabled = true;
      el.btnPartyQueueSong.textContent = "Downloading...";
    }

    try {
      const res = await fetch("/api/meowcaller/party/play", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query, target }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      showToast(`Added "${data.track?.title || query}" to Party!`, "success");
      if (el.partySongInput) el.partySongInput.value = "";
      if (data.status) updateMeowCallerStatusUI(data.status);
    } catch (err) {
      showToast(`Song error: ${err.message}`, "danger");
    } finally {
      if (el.btnPartyQueueSong) {
        el.btnPartyQueueSong.disabled = false;
        el.btnPartyQueueSong.textContent = "Queue Song";
      }
    }
  }

  async function controlParty(action) {
    try {
      const res = await fetch("/api/meowcaller/party/control", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      if (data.status) updateMeowCallerStatusUI(data.status);
    } catch (err) {
      showToast(`Control error: ${err.message}`, "danger");
    }
  }

  async function removePartyTrack(index) {
    try {
      const res = await fetch("/api/meowcaller/party/queue/remove", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ index }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      showToast(`Removed "${data.removed?.title || 'track'}" from queue`);
      if (data.status) updateMeowCallerStatusUI(data.status);
    } catch (err) {
      showToast(`Remove error: ${err.message}`, "danger");
    }
  }

  // Live Call Audio Monitor (Web Audio API + WebSocket)

  let audioContext = null;
  let audioSocket = null;
  let isListening = false;
  let isAudioMuted = false;
  let currentFeedChannel = "mixed"; // "mixed", "incoming", "outgoing"
  let masterGainNode = null;
  let incomingGainNode = null;
  let outgoingGainNode = null;
  let compressorNode = null;
  let nextPlayTimeIncoming = 0;
  let nextPlayTimeOutgoing = 0;
  let incomingHistory = new Array(36).fill(0);
  let outgoingHistory = new Array(36).fill(0);
  let incomingLastActiveTime = 0;
  let outgoingLastActiveTime = 0;
  let totalPacketsReceived = 0;
  let animFrameId = null;

  async function startAudioMonitoring() {
    try {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!AudioCtx) {
        showToast("Web Audio API is not supported in this browser", "warning");
        return;
      }

      // Initialize AudioContext without forcing 16000Hz hardware rate
      // Windows audio devices (Realtek, WASAPI, Bluetooth) require native rate (44.1k/48k)
      // Web Audio API automatically resamples our 16000Hz buffer to hardware rate cleanly
      if (!audioContext || audioContext.state === "closed") {
        audioContext = new AudioCtx();
      }

      if (audioContext.state === "suspended") {
        await audioContext.resume().catch(() => { });
      }

      // Disconnect and rebuild gain nodes cleanly
      if (incomingGainNode) { try { incomingGainNode.disconnect(); } catch (_) { } }
      if (outgoingGainNode) { try { outgoingGainNode.disconnect(); } catch (_) { } }
      if (compressorNode) { try { compressorNode.disconnect(); } catch (_) { } }
      if (masterGainNode) { try { masterGainNode.disconnect(); } catch (_) { } }

      masterGainNode = audioContext.createGain();
      incomingGainNode = audioContext.createGain();
      outgoingGainNode = audioContext.createGain();

      // Dynamics compressor for AGC: boosts soft caller speech, prevents harsh clipping
      compressorNode = audioContext.createDynamicsCompressor();
      compressorNode.threshold.setValueAtTime(-24, audioContext.currentTime);
      compressorNode.knee.setValueAtTime(10, audioContext.currentTime);
      compressorNode.ratio.setValueAtTime(5, audioContext.currentTime);
      compressorNode.attack.setValueAtTime(0.003, audioContext.currentTime);
      compressorNode.release.setValueAtTime(0.25, audioContext.currentTime);

      const masterVol = parseFloat(el.sliderMasterVolume?.value || "1");
      const incomingGain = parseFloat(el.sliderGainIncoming?.value || "2.2");
      const outgoingGain = parseFloat(el.sliderGainOutgoing?.value || "1");

      masterGainNode.gain.setValueAtTime(isAudioMuted ? 0 : masterVol, audioContext.currentTime);
      incomingGainNode.gain.setValueAtTime(incomingGain, audioContext.currentTime);
      outgoingGainNode.gain.setValueAtTime(outgoingGain, audioContext.currentTime);

      // Inbound voice routing: incomingGain -> AGC compressor -> masterGain -> destination
      incomingGainNode.connect(compressorNode);
      compressorNode.connect(masterGainNode);

      // Outbound bot/music routing: outgoingGain -> masterGain -> destination
      outgoingGainNode.connect(masterGainNode);
      masterGainNode.connect(audioContext.destination);

      if (audioSocket && (audioSocket.readyState === WebSocket.OPEN || audioSocket.readyState === WebSocket.CONNECTING)) {
        isListening = true;
        updateAudioMonitorUI(true);
        return;
      }

      if (audioSocket) {
        try { audioSocket.close(); } catch (_) { }
      }

      const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
      const wsUrl = `${proto}//${window.location.host}/api/meowcaller/audio/ws`;
      audioSocket = new WebSocket(wsUrl);
      audioSocket.binaryType = "arraybuffer";
      isListening = true;

      audioSocket.onopen = () => {
        isListening = true;
        totalPacketsReceived = 0;
        nextPlayTimeIncoming = 0;
        nextPlayTimeOutgoing = 0;
        updateAudioMonitorUI(true);
        startVisualizerLoops();
        showToast("Connected to Live Call Audio Monitor! Speaker audio active.", "success");
      };

      audioSocket.onmessage = (event) => {
        if (!isListening || !audioContext) return;
        const buffer = event.data;
        if (buffer.byteLength < 3) return;

        const view = new DataView(buffer);
        const channelTag = view.getUint8(0); // 1 = Outgoing/Bot, 2 = Incoming/Callers
        const sampleCount = Math.floor((buffer.byteLength - 1) / 2);

        // Feed filter check
        if (currentFeedChannel === "incoming" && channelTag !== 2) return;
        if (currentFeedChannel === "outgoing" && channelTag !== 1) return;

        const audioBuffer = audioContext.createBuffer(1, sampleCount, 16000);
        const channelData = audioBuffer.getChannelData(0);
        let sumSquares = 0;

        for (let i = 0; i < sampleCount; i++) {
          const s16 = view.getInt16(1 + i * 2, true); // little-endian
          const f32 = s16 / 32768.0;
          channelData[i] = f32;
          sumSquares += f32 * f32;
        }

        const rms = Math.sqrt(sumSquares / sampleCount);
        const level = Math.min(1, rms * 4.5);

        const now = Date.now();
        if (channelTag === 2) {
          incomingHistory.push(level);
          incomingHistory.shift();
          if (level > 0.03) incomingLastActiveTime = now;
        } else if (channelTag === 1) {
          outgoingHistory.push(level);
          outgoingHistory.shift();
          if (level > 0.03) outgoingLastActiveTime = now;
        }

        // Wake up audio context if suspended by browser autoplay policy
        if (audioContext.state === "suspended") {
          audioContext.resume().catch(() => { });
        }

        const source = audioContext.createBufferSource();
        source.buffer = audioBuffer;

        const curTime = audioContext.currentTime;
        const leadTime = 0.035;

        // Separate timelines for incoming caller voices and outgoing music so they play
        // in parallel simultaneously without clock-doubling latency drift or interleaving gaps.
        if (channelTag === 2) {
          source.connect(incomingGainNode);
          if (nextPlayTimeIncoming < curTime || nextPlayTimeIncoming > curTime + 1.5) {
            nextPlayTimeIncoming = curTime + leadTime;
          }
          source.start(nextPlayTimeIncoming);
          nextPlayTimeIncoming += audioBuffer.duration;
        } else {
          source.connect(outgoingGainNode);
          if (nextPlayTimeOutgoing < curTime || nextPlayTimeOutgoing > curTime + 1.5) {
            nextPlayTimeOutgoing = curTime + leadTime;
          }
          source.start(nextPlayTimeOutgoing);
          nextPlayTimeOutgoing += audioBuffer.duration;
        }

        totalPacketsReceived++;

        if (totalPacketsReceived % 4 === 0 && el.audioMonitorStats) {
          const ctxState = audioContext.state.toUpperCase();
          const channelName = channelTag === 2 ? "INBOUND CALLER" : "OUTBOUND BOT";
          const peakDb = rms > 0.0001 ? Math.round(20 * Math.log10(rms)) : -60;
          el.audioMonitorStats.textContent = `16kHz PCM • Pkts: ${totalPacketsReceived} • Last: ${channelName} (${peakDb}dB) • Audio: ${ctxState}`;
        }
      };

      audioSocket.onerror = (err) => {
        console.warn("[AudioMonitor] WebSocket error:", err);
      };

      audioSocket.onclose = () => {
        if (isListening) {
          console.log("[AudioMonitor] WebSocket closed. Auto-reconnecting in 2s...");
          setTimeout(() => {
            if (isListening) startAudioMonitoring();
          }, 2000);
        }
      };

    } catch (err) {
      showToast(`Audio monitor start error: ${err.message}`, "danger");
      stopAudioMonitoring();
    }
  }

  function stopAudioMonitoring() {
    isListening = false;
    if (audioSocket) {
      try { audioSocket.close(); } catch (_) { }
      audioSocket = null;
    }
    if (audioContext && audioContext.state !== "closed") {
      try { audioContext.suspend().catch(() => { }); } catch (_) { }
    }
    if (animFrameId) {
      cancelAnimationFrame(animFrameId);
      animFrameId = null;
    }
    updateAudioMonitorUI(false);
    drawVisualizerIdle();
  }

  function toggleAudioMonitoring() {
    if (isListening) {
      stopAudioMonitoring();
      showToast("Live call audio monitor stopped");
    } else {
      startAudioMonitoring();
    }
  }

  function testAudioOutput() {
    try {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!AudioCtx) {
        showToast("Web Audio API is not supported in this browser", "warning");
        return;
      }
      if (!audioContext || audioContext.state === "closed") {
        audioContext = new AudioCtx();
      }
      audioContext.resume().then(() => {
        // Melodic 2-tone test chime (C5 523Hz -> G5 784Hz)
        const now = audioContext.currentTime;
        const osc1 = audioContext.createOscillator();
        const osc2 = audioContext.createOscillator();
        const testGain = audioContext.createGain();

        osc1.type = "sine";
        osc2.type = "sine";
        osc1.frequency.setValueAtTime(523.25, now);
        osc2.frequency.setValueAtTime(783.99, now + 0.12);

        testGain.gain.setValueAtTime(0, now);
        testGain.gain.linearRampToValueAtTime(0.35, now + 0.02);
        testGain.gain.exponentialRampToValueAtTime(0.001, now + 0.35);

        osc1.connect(testGain);
        osc2.connect(testGain);
        testGain.connect(audioContext.destination);

        osc1.start(now);
        osc1.stop(now + 0.14);
        osc2.start(now + 0.12);
        osc2.stop(now + 0.36);

        showToast("Test chime played! AudioContext is running and speakers are active.", "success");
        if (!isListening) {
          startAudioMonitoring();
        }
      }).catch((err) => {
        showToast(`Speaker test error: ${err.message}`, "danger");
      });
    } catch (err) {
      showToast(`Speaker test error: ${err.message}`, "danger");
    }
  }

  function updateAudioMonitorUI(active) {
    if (el.audioMonitorLiveDot) {
      el.audioMonitorLiveDot.classList.toggle("active", active);
    }
    if (el.audioMonitorBadge) {
      el.audioMonitorBadge.textContent = active ? "LISTENING LIVE" : "STANDBY";
      el.audioMonitorBadge.className = active ? "badge badge-success" : "badge badge-neutral";
    }
    if (el.audioListenBtnIcon) {
      el.audioListenBtnIcon.innerHTML = active ? '<svg class="icon-inline"><use href="#icon-square"/></svg>' : '<svg class="icon-inline"><use href="#icon-headphones"/></svg>';
    }
    if (el.audioListenBtnText) {
      el.audioListenBtnText.textContent = active ? "Stop Listening" : "Start Listening";
    }
    if (!active && el.audioMonitorStats) {
      el.audioMonitorStats.textContent = "16kHz PCM • Web Audio API • Ultra-Low Latency";
    }
  }

  function startVisualizerLoops() {
    function render() {
      if (!isListening) return;

      const now = Date.now();
      const incomingActive = (now - incomingLastActiveTime) < 350;
      const outgoingActive = (now - outgoingLastActiveTime) < 350;

      if (el.badgeIncomingVoiceActivity) {
        el.badgeIncomingVoiceActivity.textContent = incomingActive ? "SPEAKING" : "IDLE";
        el.badgeIncomingVoiceActivity.style.background = incomingActive ? "rgba(56, 189, 248, 0.35)" : "rgba(56, 189, 248, 0.15)";
      }
      if (el.badgeOutgoingVoiceActivity) {
        el.badgeOutgoingVoiceActivity.textContent = outgoingActive ? "STREAMING" : "IDLE";
        el.badgeOutgoingVoiceActivity.style.background = outgoingActive ? "rgba(168, 85, 247, 0.35)" : "rgba(168, 85, 247, 0.15)";
      }

      drawWaveform(el.canvasIncomingAudio, incomingHistory, "#38bdf8", "#0284c7");
      drawWaveform(el.canvasOutgoingAudio, outgoingHistory, "#c084fc", "#9333ea");

      animFrameId = requestAnimationFrame(render);
    }

    if (animFrameId) cancelAnimationFrame(animFrameId);
    animFrameId = requestAnimationFrame(render);
  }

  function drawWaveform(canvas, history, colorPrimary, colorGlow) {
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const w = canvas.width;
    const h = canvas.height;
    ctx.clearRect(0, 0, w, h);

    const barWidth = Math.floor(w / history.length) - 2;
    const centerY = h / 2;

    for (let i = 0; i < history.length; i++) {
      const val = history[i];
      const barH = Math.max(3, val * (h - 8));
      const x = i * (barWidth + 2);
      const y = centerY - barH / 2;

      ctx.fillStyle = colorPrimary;
      ctx.shadowBlur = val > 0.1 ? 6 : 0;
      ctx.shadowColor = colorGlow;

      ctx.beginPath();
      ctx.roundRect ? ctx.roundRect(x, y, barWidth, barH, 2) : ctx.rect(x, y, barWidth, barH);
      ctx.fill();
    }
    ctx.shadowBlur = 0;
  }

  function drawVisualizerIdle() {
    drawWaveform(el.canvasIncomingAudio, new Array(36).fill(0.04), "#475569", "transparent");
    drawWaveform(el.canvasOutgoingAudio, new Array(36).fill(0.04), "#475569", "transparent");
    if (el.badgeIncomingVoiceActivity) el.badgeIncomingVoiceActivity.textContent = "IDLE";
    if (el.badgeOutgoingVoiceActivity) el.badgeOutgoingVoiceActivity.textContent = "IDLE";
  }

  // Radio Station DJ & Live Broadcast Mic

  let isRadioOnAir = false;
  let isRadioPttActive = false;
  let radioMode = "ducking"; // "ducking" | "mic_only"
  let radioMicGain = 1.0;
  let radioDuckLevel = 0.15;
  let radioMediaStream = null;
  let radioAudioContext = null;
  let radioSourceNode = null;
  let radioProcessorNode = null;
  let radioResampleBuffer = [];

  async function populateRadioMicDevices() {
    if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) return;
    try {
      const devices = await navigator.mediaDevices.enumerateDevices();
      const audioInputs = devices.filter((d) => d.kind === "audioinput");
      if (el.selectRadioMicDevice && audioInputs.length > 0) {
        const cur = el.selectRadioMicDevice.value;
        el.selectRadioMicDevice.innerHTML = audioInputs.map((d, idx) => {
          const label = d.label || `Microphone ${idx + 1}`;
          return `<option value="${escapeHtml(d.deviceId)}">${escapeHtml(label)}</option>`;
        }).join("");
        if (cur && audioInputs.some((d) => d.deviceId === cur)) {
          el.selectRadioMicDevice.value = cur;
        }
      }
    } catch (_) { }
  }

  function ensureAudioSocketForMic() {
    if (audioSocket && audioSocket.readyState === WebSocket.OPEN) {
      return audioSocket;
    }
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${proto}//${window.location.host}/api/meowcaller/audio/ws`;
    const ws = new WebSocket(wsUrl);
    ws.binaryType = "arraybuffer";
    ws.onopen = () => {
      audioSocket = ws;
      sendRadioControl("set_mic", { active: isRadioOnAir || isRadioPttActive });
    };
    audioSocket = ws;
    return ws;
  }

  function sendRadioControl(action, payload = {}) {
    const data = Object.assign({ action }, payload);
    const ws = ensureAudioSocketForMic();
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(data));
    } else {
      fetch("/api/meowcaller/radio/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }).catch(() => { });
    }
  }

  async function startRadioMicStream() {
    try {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!radioAudioContext || radioAudioContext.state === "closed") {
        radioAudioContext = new AudioCtx();
      }
      if (radioAudioContext.state === "suspended") {
        await radioAudioContext.resume().catch(() => { });
      }

      const selectedDeviceId = el.selectRadioMicDevice?.value;
      const constraints = {
        audio: {
          deviceId: selectedDeviceId && selectedDeviceId !== "default" ? { exact: selectedDeviceId } : undefined,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      };

      radioMediaStream = await navigator.mediaDevices.getUserMedia(constraints);
      populateRadioMicDevices();

      radioSourceNode = radioAudioContext.createMediaStreamSource(radioMediaStream);

      const inputSampleRate = radioAudioContext.sampleRate;
      const targetSampleRate = 16000;
      const resampleRatio = inputSampleRate / targetSampleRate;

      radioProcessorNode = radioAudioContext.createScriptProcessor(4096, 1, 1);
      radioResampleBuffer = [];

      radioProcessorNode.onaudioprocess = (e) => {
        const active = isRadioOnAir || isRadioPttActive;
        const inputChannel = e.inputBuffer.getChannelData(0);

        let sumSq = 0;
        for (let i = 0; i < inputChannel.length; i++) {
          sumSq += inputChannel[i] * inputChannel[i];
        }
        const rms = Math.sqrt(sumSq / inputChannel.length);
        updateRadioVuMeter(rms, active);

        if (!active) return;

        const outSamplesCount = Math.floor(inputChannel.length / resampleRatio);
        for (let outIdx = 0; outIdx < outSamplesCount; outIdx++) {
          const inIdx = outIdx * resampleRatio;
          const i0 = Math.floor(inIdx);
          const i1 = Math.min(i0 + 1, inputChannel.length - 1);
          const frac = inIdx - i0;
          const sample = inputChannel[i0] * (1 - frac) + inputChannel[i1] * frac;
          radioResampleBuffer.push(sample);
        }

        while (radioResampleBuffer.length >= 960) {
          const chunk = radioResampleBuffer.slice(0, 960);
          radioResampleBuffer = radioResampleBuffer.slice(960);

          const packet = new Uint8Array(1 + 960 * 2);
          packet[0] = 3; // ChannelMic tag
          const view = new DataView(packet.buffer);
          for (let i = 0; i < 960; i++) {
            let s = chunk[i] * 32768.0;
            if (s > 32767) s = 32767;
            else if (s < -32768) s = -32768;
            view.setInt16(1 + i * 2, Math.round(s), true);
          }

          const ws = ensureAudioSocketForMic();
          if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(packet.buffer);
          }
        }
      };

      radioSourceNode.connect(radioProcessorNode);

      const dummyGain = radioAudioContext.createGain();
      dummyGain.gain.value = 0;
      radioProcessorNode.connect(dummyGain);
      dummyGain.connect(radioAudioContext.destination);

      return true;
    } catch (err) {
      showToast(`Microphone permission error: ${err.message}`, "danger");
      return false;
    }
  }

  function stopRadioMicStream() {
    if (radioProcessorNode) {
      try { radioProcessorNode.disconnect(); } catch (_) { }
      radioProcessorNode = null;
    }
    if (radioSourceNode) {
      try { radioSourceNode.disconnect(); } catch (_) { }
      radioSourceNode = null;
    }
    if (radioMediaStream) {
      radioMediaStream.getTracks().forEach((t) => t.stop());
      radioMediaStream = null;
    }
    updateRadioVuMeter(0, false);
  }

  function updateRadioVuMeter(rms, active) {
    if (!el.radioVuBar) return;
    const db = rms > 0.0001 ? 20 * Math.log10(rms) : -60;
    const pct = Math.max(0, Math.min(100, ((db + 60) / 60) * 100));
    el.radioVuBar.style.width = `${pct.toFixed(1)}%`;

    if (el.labelRadioMicDb) {
      el.labelRadioMicDb.textContent = db > -59 ? `${Math.round(db)} dB` : "-60 dB";
    }

    const isSpeaking = active && db > -38;
    if (el.radioSpeakingBadge) {
      el.radioSpeakingBadge.style.display = isSpeaking ? "inline-flex" : "none";
    }
  }

  function updateRadioUI(active) {
    if (el.radioLiveDot) {
      el.radioLiveDot.className = active ? "live-dot active" : "live-dot";
      el.radioLiveDot.style.background = active ? "#ef4444" : "#64748b";
    }
    if (el.radioOnAirBadge) {
      el.radioOnAirBadge.innerHTML = active ? '<span class="status-dot" style="background-color: var(--danger);"></span> ON AIR' : 'OFF AIR';
      el.radioOnAirBadge.className = active ? "badge badge-danger" : "badge badge-neutral";
    }
    if (el.btnRadioMicToggle) {
      el.btnRadioMicToggle.className = active ? "btn btn-sm btn-outline" : "btn btn-sm btn-danger";
      if (el.radioMicBtnIcon) el.radioMicBtnIcon.innerHTML = active ? '<svg class="icon-inline"><use href="#icon-square"/></svg>' : '<svg class="icon-inline"><use href="#icon-mic"/></svg>';
      if (el.radioMicBtnText) el.radioMicBtnText.textContent = active ? "LEAVE AIR" : "GO ON AIR";
    }
    if (el.radioStatusText) {
      if (active) {
        el.radioStatusText.textContent = radioMode === "ducking"
          ? "LIVE ON AIR! Music ducking active (speaking over music)."
          : "LIVE ON AIR! Music muted while speaking.";
        el.radioStatusText.style.color = "#f87171";
      } else {
        el.radioStatusText.textContent = "Microphone standby. Click 'GO ON AIR' or hold PTT to speak.";
        el.radioStatusText.style.color = "var(--text-secondary)";
      }
    }
  }

  async function toggleRadioOnAir() {
    if (isRadioOnAir) {
      isRadioOnAir = false;
      stopRadioMicStream();
      sendRadioControl("set_mic", { active: false });
      updateRadioUI(false);
      showToast("Radio DJ mic OFF AIR (Music restored to 100%)", "info");
    } else {
      const ok = await startRadioMicStream();
      if (!ok) return;
      isRadioOnAir = true;
      sendRadioControl("set_mic", {
        active: true,
        mode: radioMode,
        duckLevel: radioDuckLevel,
        micGain: radioMicGain,
      });
      updateRadioUI(true);
      showToast("LIVE ON AIR! Broadcast mic is live into the WhatsApp call.", "success");
    }
  }

  async function setRadioPTT(active) {
    if (isRadioOnAir) return;
    if (active === isRadioPttActive) return;
    isRadioPttActive = active;

    if (el.btnRadioPTT) {
      el.btnRadioPTT.className = active ? "btn btn-sm btn-danger" : "btn btn-sm btn-outline";
    }
    if (el.radioPttText) {
      el.radioPttText.textContent = active ? "TALKING (Release to End)" : "Hold to Talk (PTT)";
    }

    if (active) {
      await startRadioMicStream();
      sendRadioControl("set_mic", {
        active: true,
        mode: radioMode,
        duckLevel: radioDuckLevel,
        micGain: radioMicGain,
      });
      updateRadioUI(true);
    } else {
      sendRadioControl("set_mic", { active: false });
      stopRadioMicStream();
      updateRadioUI(false);
    }
  }

  // Realtime AI VoiceBot Logic

  async function fetchVoiceBotVoices() {
    try {
      const res = await fetch("/api/meowcaller/voicebot/voices");
      if (!res.ok) return;
      const data = await res.json();
      if (data && data.voices && el.voiceBotVoiceSelect) {
        const cur = el.voiceBotVoiceSelect.value;
        el.voiceBotVoiceSelect.innerHTML = data.voices.map((v) => {
          const label = v.replace(/_/g, " ").replace(/\b\w/g, l => l.toUpperCase());
          return `<option value="${escapeHtml(v)}">${escapeHtml(label)}</option>`;
        }).join("");
        if (cur) el.voiceBotVoiceSelect.value = cur;
      }
    } catch (_) { }
  }

  async function connectVoiceBotWs() {
    const serverUrl = el.voiceBotServerUrl?.value?.trim() || "ws://127.0.0.1:6006";
    if (el.btnConnectVoiceBotWs) {
      el.btnConnectVoiceBotWs.disabled = true;
      if (el.btnConnectWsText) el.btnConnectWsText.textContent = "Connecting...";
      if (el.btnConnectWsIcon) el.btnConnectWsIcon.innerHTML = '<svg class="icon-inline spin"><use href="#icon-refresh"/></svg>';
    }
    if (el.voiceBotWsError) {
      el.voiceBotWsError.style.display = "none";
      el.voiceBotWsError.textContent = "";
    }

    try {
      const res = await fetch("/api/meowcaller/voicebot/connect", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ serverUrl }),
      });
      const data = await res.json();
      if (!res.ok || !data.success) {
        throw new Error(data.error || `HTTP ${res.status}`);
      }

      if (el.voiceBotWsConnBadge) {
        el.voiceBotWsConnBadge.textContent = "Groq Whisper STT";
        el.voiceBotWsConnBadge.className = "badge badge-success";
      }
      if (el.btnConnectWsText) el.btnConnectWsText.textContent = "Active ✓";
      if (el.btnConnectWsIcon) el.btnConnectWsIcon.innerHTML = '<svg class="icon-inline" style="color: var(--success);"><use href="#icon-check"/></svg>';
      showToast(`Configured Groq Whisper STT`, "success");
      if (data.status) updateMeowCallerStatusUI(data.status);
    } catch (err) {
      if (el.voiceBotWsConnBadge) {
        const vbEnabled = !!(state.meowcaller?.voiceBot?.enabled);
        el.voiceBotWsConnBadge.textContent = vbEnabled ? "Cloud ASR (Groq / Gemini)" : "Disconnected";
        el.voiceBotWsConnBadge.className = vbEnabled ? "badge badge-primary" : "badge badge-neutral";
      }
      if (el.voiceBotWsError) {
        el.voiceBotWsError.style.display = "inline";
        el.voiceBotWsError.textContent = err.message;
      }
      if (el.btnConnectWsText) el.btnConnectWsText.textContent = "Connect WSS";
      if (el.btnConnectWsIcon) el.btnConnectWsIcon.innerHTML = '<svg class="icon-inline"><use href="#icon-zap"/></svg>';
      showToast(`WebSocket connection failed: ${err.message}`, "danger");
    } finally {
      if (el.btnConnectVoiceBotWs) el.btnConnectVoiceBotWs.disabled = false;
    }
  }

  async function fetchVoiceBotConfig() {
    try {
      const res = await fetch("/api/meowcaller/voicebot/config");
      if (!res.ok) return;
      const data = await res.json();
      if (data && data.voiceBot) {
        const vb = data.voiceBot;
        if (el.voiceBotServerUrl && vb.serverUrl) el.voiceBotServerUrl.value = vb.serverUrl;
        if (el.voiceBotModelSelect && vb.geminiModel) el.voiceBotModelSelect.value = vb.geminiModel;
        if (el.voiceBotVoiceSelect && vb.voice) el.voiceBotVoiceSelect.value = vb.voice;
        if (el.voiceBotSilenceSlider && vb.silenceThresholdMs) {
          el.voiceBotSilenceSlider.value = vb.silenceThresholdMs;
          if (el.voiceBotSilenceVal) el.voiceBotSilenceVal.textContent = `${vb.silenceThresholdMs} ms`;
        }
        if (el.voiceBotSongTriggerInput && vb.songTriggerPhrase) el.voiceBotSongTriggerInput.value = vb.songTriggerPhrase;
        if (el.voiceBotPromptInput && vb.systemPrompt) el.voiceBotPromptInput.value = vb.systemPrompt;
      }
    } catch (_) { }
  }

  async function toggleVoiceBotMaster() {
    const currentBtnText = el.voiceBotBtnText?.textContent || "";
    const isCurrentlyEnabled = currentBtnText.includes("Disable") ||
      !!(state.meowcaller && state.meowcaller.voiceBot && state.meowcaller.voiceBot.enabled);
    const newEnabled = !isCurrentlyEnabled;

    if (el.btnToggleVoiceBot) {
      el.btnToggleVoiceBot.disabled = true;
      if (el.voiceBotBtnText) el.voiceBotBtnText.textContent = newEnabled ? "Enabling..." : "Disabling...";
    }

    const serverUrl = el.voiceBotServerUrl?.value?.trim() || "ws://127.0.0.1:6006";
    const voice = el.voiceBotVoiceSelect?.value || "elon_musk";
    const geminiModel = el.voiceBotModelSelect?.value || "gemini-3.5-flash-lite";
    const silenceThresholdMs = parseInt(el.voiceBotSilenceSlider?.value || "700", 10);
    const songTriggerPhrase = el.voiceBotSongTriggerInput?.value?.trim() || "play song";
    const systemPrompt = el.voiceBotPromptInput?.value?.trim() || "";

    try {
      const res = await fetch("/api/meowcaller/voicebot/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled: newEnabled,
          serverUrl,
          voice,
          geminiModel,
          silenceThresholdMs,
          songTriggerPhrase,
          systemPrompt,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);

      if (!state.meowcaller) state.meowcaller = {};
      if (!state.meowcaller.voiceBot) state.meowcaller.voiceBot = {};
      state.meowcaller.voiceBot.enabled = newEnabled;

      if (data.status) {
        updateMeowCallerStatusUI(data.status);
      } else {
        if (el.btnToggleVoiceBot) {
          el.btnToggleVoiceBot.className = newEnabled ? "btn btn-sm btn-danger" : "btn btn-sm btn-primary";
          if (el.voiceBotBtnIcon) el.voiceBotBtnIcon.innerHTML = newEnabled ? '<svg class="icon-inline"><use href="#icon-square"/></svg>' : '<svg class="icon-inline"><use href="#icon-mic"/></svg>';
          if (el.voiceBotBtnText) el.voiceBotBtnText.textContent = newEnabled ? "Disable VoiceBot" : "Enable VoiceBot";
        }
        if (el.voiceBotStatusBadge) {
          el.voiceBotStatusBadge.textContent = newEnabled ? "LISTENING" : "STANDBY";
          el.voiceBotStatusBadge.className = newEnabled ? "badge badge-primary" : "badge badge-neutral";
        }
      }
      showToast(newEnabled ? "Realtime AI VoiceBot ENABLED! Listening for call speech." : "VoiceBot DISABLED", newEnabled ? "success" : "info");
    } catch (err) {
      showToast(`VoiceBot toggle error: ${err.message}`, "danger");
      if (el.voiceBotBtnText) {
        el.voiceBotBtnText.textContent = isCurrentlyEnabled ? "Disable VoiceBot" : "Enable VoiceBot";
      }
    } finally {
      if (el.btnToggleVoiceBot) el.btnToggleVoiceBot.disabled = false;
    }
  }

  async function saveVoiceBotConfig() {
    const isCurrentlyEnabled = (el.voiceBotBtnText?.textContent || "").includes("Disable") ||
      !!(state.meowcaller && state.meowcaller.voiceBot && state.meowcaller.voiceBot.enabled);
    const serverUrl = el.voiceBotServerUrl?.value?.trim() || "ws://127.0.0.1:6006";
    const voice = el.voiceBotVoiceSelect?.value || "elon_musk";
    const geminiModel = el.voiceBotModelSelect?.value || "gemini-3.5-flash-lite";
    const silenceThresholdMs = parseInt(el.voiceBotSilenceSlider?.value || "700", 10);
    const songTriggerPhrase = el.voiceBotSongTriggerInput?.value?.trim() || "play song";
    const systemPrompt = el.voiceBotPromptInput?.value?.trim() || "";

    try {
      const res = await fetch("/api/meowcaller/voicebot/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled: isCurrentlyEnabled,
          serverUrl,
          voice,
          geminiModel,
          silenceThresholdMs,
          songTriggerPhrase,
          systemPrompt,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      if (data.status) updateMeowCallerStatusUI(data.status);
      showToast("VoiceBot configuration saved!", "success");
    } catch (err) {
      showToast(`Save error: ${err.message}`, "danger");
    }
  }

  // Event Handlers & Submissions

  function initEventHandlers() {
    // Realtime AI VoiceBot Listeners
    if (el.voiceBotSilenceSlider) {
      el.voiceBotSilenceSlider.addEventListener("input", (e) => {
        if (el.voiceBotSilenceVal) {
          el.voiceBotSilenceVal.textContent = `${e.target.value} ms`;
        }
      });
    }

    if (el.btnConnectVoiceBotWs) {
      el.btnConnectVoiceBotWs.addEventListener("click", connectVoiceBotWs);
    }

    if (el.btnToggleVoiceBot) {
      el.btnToggleVoiceBot.addEventListener("click", toggleVoiceBotMaster);
    }

    if (el.btnSaveVoiceBotConfig) {
      el.btnSaveVoiceBotConfig.addEventListener("click", saveVoiceBotConfig);
    }

    // Live Call Audio Monitor Listeners
    if (el.btnToggleAudioListen) {
      el.btnToggleAudioListen.addEventListener("click", toggleAudioMonitoring);
    }

    if (el.btnQuickStartListening) {
      el.btnQuickStartListening.addEventListener("click", toggleAudioMonitoring);
    }

    if (el.btnTestAudioChime) {
      el.btnTestAudioChime.addEventListener("click", testAudioOutput);
    }

    // Auto-resume AudioContext on any user interaction to satisfy browser autoplay policy
    document.addEventListener("click", () => {
      if (audioContext && audioContext.state === "suspended") {
        audioContext.resume().catch(() => { });
      }
    }, { passive: true });

    if (el.btnAudioMuteToggle) {
      el.btnAudioMuteToggle.addEventListener("click", () => {
        isAudioMuted = !isAudioMuted;
        if (el.audioMuteIcon) {
          el.audioMuteIcon.innerHTML = isAudioMuted ? '<svg class="icon-inline"><use href="#icon-volume-x"/></svg>' : '<svg class="icon-inline"><use href="#icon-volume"/></svg>';
        }
        if (masterGainNode) {
          const vol = parseFloat(el.sliderMasterVolume?.value || "1");
          masterGainNode.gain.value = isAudioMuted ? 0 : vol;
        }
      });
    }

    if (el.sliderMasterVolume) {
      el.sliderMasterVolume.addEventListener("input", (e) => {
        const val = parseFloat(e.target.value);
        if (el.labelMasterVolume) el.labelMasterVolume.textContent = `${Math.round(val * 100)}%`;
        if (masterGainNode && !isAudioMuted) {
          masterGainNode.gain.value = val;
        }
      });
    }

    if (el.sliderGainIncoming) {
      el.sliderGainIncoming.addEventListener("input", (e) => {
        const val = parseFloat(e.target.value);
        if (el.labelGainIncoming) el.labelGainIncoming.textContent = `${Math.round(val * 100)}%`;
        if (incomingGainNode) {
          incomingGainNode.gain.value = val;
        }
      });
    }

    if (el.sliderGainOutgoing) {
      el.sliderGainOutgoing.addEventListener("input", (e) => {
        const val = parseFloat(e.target.value);
        if (el.labelGainOutgoing) el.labelGainOutgoing.textContent = `${Math.round(val * 100)}%`;
        if (outgoingGainNode) {
          outgoingGainNode.gain.value = val;
        }
      });
    }

    if (el.audioFeedSelector) {
      el.audioFeedSelector.querySelectorAll(".duration-pill").forEach((btn) => {
        btn.addEventListener("click", () => {
          el.audioFeedSelector.querySelectorAll(".duration-pill").forEach((b) => b.classList.remove("active"));
          btn.classList.add("active");
          currentFeedChannel = btn.getAttribute("data-channel") || "mixed";
          if (audioContext) {
            const curTime = audioContext.currentTime;
            nextPlayTimeIncoming = curTime + 0.025;
            nextPlayTimeOutgoing = curTime + 0.025;
          }
          showToast(`Audio monitor switched to: ${btn.textContent.trim()}`);
        });
      });
    }

    // Initial idle visualizer render
    drawVisualizerIdle();
    // Tab switching
    document.querySelectorAll(".nav-btn").forEach((btn) => {
      btn.addEventListener("click", () => {
        const tab = btn.getAttribute("data-tab");
        if (tab) switchTab(tab);
      });
    });

    // Mobile menu toggle
    if (el.mobileMenuBtn && el.appSidebar) {
      el.mobileMenuBtn.addEventListener("click", () => {
        el.appSidebar.classList.toggle("open");
      });
    }

    // Theme toggle
    if (el.btnToggleTheme) {
      el.btnToggleTheme.addEventListener("click", toggleTheme);
    }

    // Power toggle
    if (el.btnToggleSleep) {
      el.btnToggleSleep.addEventListener("click", toggleSleep);
    }

    // Quick Refresh
    if (el.btnQuickRefresh) {
      el.btnQuickRefresh.addEventListener("click", () => {
        loadCurrentTabData();
        showToast("Refreshed state", "success");
      });
    }
    if (el.btnRefreshStatus) {
      el.btnRefreshStatus.addEventListener("click", () => {
        fetchStatus();
        showToast("Checked connection state", "success");
      });
    }

    // Search Chats
    if (el.searchChatsInput) {
      el.searchChatsInput.addEventListener("input", renderChatsTable);
    }

    // Open Help / Commands Modal
    if (el.btnOpenHelpModal) {
      el.btnOpenHelpModal.addEventListener("click", () => {
        if (el.helpSearchInput) el.helpSearchInput.value = "";
        filterHelpCommands("all", "");
        openModal(el.modalHelp);
      });
    }

    if (el.helpFilterPills) {
      el.helpFilterPills.querySelectorAll(".duration-pill").forEach((pill) => {
        pill.addEventListener("click", () => {
          el.helpFilterPills.querySelectorAll(".duration-pill").forEach((p) => p.classList.remove("active"));
          pill.classList.add("active");
          filterHelpCommands(pill.dataset.cat || "all", el.helpSearchInput?.value || "");
        });
      });
    }

    if (el.helpSearchInput) {
      el.helpSearchInput.addEventListener("input", (e) => {
        const activePill = el.helpFilterPills?.querySelector(".duration-pill.active");
        filterHelpCommands(activePill?.dataset.cat || "all", e.target.value);
      });
    }

    function filterHelpCommands(cat, query) {
      const q = query.toLowerCase().trim();
      const sections = el.helpCommandsList?.querySelectorAll(".help-section");
      if (!sections) return;
      sections.forEach((sec) => {
        const secCat = sec.dataset.section;
        const matchesCat = cat === "all" || secCat === cat;
        if (!matchesCat) {
          sec.style.display = "none";
          return;
        }

        const cards = sec.querySelectorAll("div[style*='background']");
        let visibleCount = 0;
        cards.forEach((card) => {
          const text = card.textContent.toLowerCase();
          const matchesQuery = !q || text.includes(q);
          card.style.display = matchesQuery ? "block" : "none";
          if (matchesQuery) visibleCount++;
        });

        sec.style.display = visibleCount > 0 ? "block" : "none";
      });
    }

    // Open Send Message Modal
    if (el.btnOpenSendMessageModal) {
      el.btnOpenSendMessageModal.addEventListener("click", () => {
        if (el.sendMsgJid) el.sendMsgJid.value = "";
        if (el.sendMsgContent) el.sendMsgContent.value = "";
        openModal(el.modalSendMessage);
      });
    }

    // Submit Send Message
    if (el.btnSubmitSendMessage) {
      el.btnSubmitSendMessage.addEventListener("click", async () => {
        const jid = el.sendMsgJid?.value?.trim();
        const text = el.sendMsgContent?.value?.trim();
        if (!jid || !text) {
          showToast("Please provide both a recipient and message text", "warning");
          return;
        }

        try {
          const res = await fetch("/api/message/send", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ jid, text }),
          });
          if (res.ok) {
            showToast("Message dispatched successfully", "success");
            closeModal(el.modalSendMessage);
          } else {
            const errText = await res.text();
            showToast(`Send failed: ${errText}`, "danger");
          }
        } catch (e) {
          showToast(`Error: ${e.message}`, "danger");
        }
      });
    }

    // Open Mute Modal
    if (el.btnOpenMuteModal) {
      el.btnOpenMuteModal.addEventListener("click", () => {
        if (el.muteTargetInput) el.muteTargetInput.value = "";
        openModal(el.modalMute);
      });
    }

    // Submit Mute
    if (el.btnSubmitMute) {
      el.btnSubmitMute.addEventListener("click", async () => {
        const user = el.muteTargetInput?.value?.trim();
        if (!user) {
          showToast("Please provide a phone number or JID", "warning");
          return;
        }

        try {
          const res = await fetch("/api/moderation/mute", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ jid: "global", senderId: user }),
          });
          if (res.ok) {
            showToast(`Muted user ${user}`, "success");
            closeModal(el.modalMute);
            fetchMutes();
          }
        } catch (e) {
          showToast("Failed to mute user", "danger");
        }
      });
    }

    // Open Reminder Modal
    if (el.btnOpenReminderModal) {
      el.btnOpenReminderModal.addEventListener("click", () => {
        if (el.reminderTextInput) el.reminderTextInput.value = "";
        openModal(el.modalReminder);
      });
    }

    // Submit Reminder
    if (el.btnSubmitReminder) {
      el.btnSubmitReminder.addEventListener("click", async () => {
        const jid = el.reminderJidInput?.value?.trim() || "global";
        const text = el.reminderTextInput?.value?.trim();
        const duration = parseInt(el.reminderDurationInput?.value || "15", 10);

        if (!text) {
          showToast("Please provide reminder text", "warning");
          return;
        }

        try {
          const res = await fetch("/api/reminders", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ jid, text, minutes: duration }),
          });
          if (res.ok) {
            showToast("Reminder scheduled", "success");
            closeModal(el.modalReminder);
            fetchReminders();
          }
        } catch (e) {
          showToast("Failed to schedule reminder", "danger");
        }
      });
    }

    // Open Add Trigger Modal
    if (el.btnAddTriggerBtn) {
      el.btnAddTriggerBtn.addEventListener("click", () => {
        if (el.triggerKeywordInput) el.triggerKeywordInput.value = "";
        if (el.triggerResponseInput) el.triggerResponseInput.value = "";
        openModal(el.modalTrigger);
      });
    }

    // Submit Add Trigger
    if (el.btnSubmitTrigger) {
      el.btnSubmitTrigger.addEventListener("click", async () => {
        const phrase = el.triggerKeywordInput?.value?.trim();
        const reply = el.triggerResponseInput?.value?.trim();
        if (!phrase || !reply) {
          showToast("Please enter both keyword and reply", "warning");
          return;
        }

        try {
          const res = await fetch("/api/autoreply", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ addTrigger: { phrase, reply } }),
          });
          if (res.ok) {
            showToast("Added auto-reply trigger", "success");
            closeModal(el.modalTrigger);
            fetchAutoReply();
          }
        } catch (e) {
          showToast("Failed to add trigger", "danger");
        }
      });
    }

    // Persona Test
    if (el.btnTestPersona) {
      el.btnTestPersona.addEventListener("click", testPersona);
    }

    // AI Global & Chat Override Handlers
    if (el.btnSaveGlobalAI) {
      el.btnSaveGlobalAI.addEventListener("click", saveGlobalModel);
    }
    if (el.btnSaveChatOverride) {
      el.btnSaveChatOverride.addEventListener("click", saveChatOverride);
    }

    // Clean Storage
    if (el.btnCleanMedia) {
      el.btnCleanMedia.addEventListener("click", cleanMediaStorage);
    }

    // Logs Controls
    if (el.logLevelFilter) {
      el.logLevelFilter.addEventListener("change", (e) => {
        state.logsFilter = e.target.value;
        renderLogs();
      });
    }
    if (el.btnToggleAutoScroll) {
      el.btnToggleAutoScroll.addEventListener("click", () => {
        state.autoScrollLogs = !state.autoScrollLogs;
        el.btnToggleAutoScroll.textContent = `Auto-Scroll: ${state.autoScrollLogs ? "ON" : "OFF"}`;
      });
    }
    if (el.btnClearLogs) {
      el.btnClearLogs.addEventListener("click", () => {
        state.logs = [];
        renderLogs();
        showToast("Cleared local log view", "success");
      });
    }

    // Audit Actions
    if (el.btnRefreshAudit) {
      el.btnRefreshAudit.addEventListener("click", fetchAudit);
    }
    if (el.btnClearAudit) {
      el.btnClearAudit.addEventListener("click", async () => {
        if (!confirm("Are you sure you want to clear all intercepted deleted messages?")) return;
        try {
          const res = await fetch("/api/audit/deleted", { method: "DELETE" });
          if (res.ok) {
            showToast("Audit Vault cleared", "success");
            fetchAudit();
          }
        } catch (e) {
          showToast(`Failed to clear audit: ${e.message}`, "danger");
        }
      });
    }

    // Reports & Deadlines
    if (el.reportDatePicker) {
      el.reportDatePicker.addEventListener("change", (e) => {
        fetchDailyReports(e.target.value);
      });
    }

    if (el.btnGenerateReportSummary) {
      el.btnGenerateReportSummary.addEventListener("click", () => {
        const date = el.reportDatePicker?.value || new Date().toISOString().split("T")[0];
        generateAISummary(date);
      });
    }

    if (el.btnAddReportGroupBtn) {
      el.btnAddReportGroupBtn.addEventListener("click", () => {
        const jid = el.newReportGroupInput?.value?.trim();
        if (!jid) {
          showToast("Please enter a group JID", "warning");
          return;
        }
        addReportGroup(jid);
      });
    }

    if (el.btnSaveDeadline) {
      el.btnSaveDeadline.addEventListener("click", saveDeadline);
    }

    if (el.btnRefreshPreview) {
      el.btnRefreshPreview.addEventListener("click", updatePFPPreview);
    }

    if (el.pfpTitleInput) {
      let previewDebounce;
      el.pfpTitleInput.addEventListener("input", () => {
        clearTimeout(previewDebounce);
        previewDebounce = setTimeout(updatePFPPreview, 300);
      });
    }

    if (el.pfpDateInput) {
      el.pfpDateInput.addEventListener("change", updatePFPPreview);
    }

    // Delegated actions for reports groups & deadlines
    document.addEventListener("click", (e) => {
      const btnRemoveGroup = e.target.closest(".btn-remove-report-group");
      if (btnRemoveGroup) {
        const jid = btnRemoveGroup.getAttribute("data-jid");
        if (confirm(`Remove group ${jid} from daily status report broadcasts?`)) {
          removeReportGroup(jid);
        }
        return;
      }

      const btnUpdatePFP = e.target.closest(".btn-update-group-pfp");
      if (btnUpdatePFP) {
        const jid = btnUpdatePFP.getAttribute("data-jid");
        forceUpdateGroupPFP(jid);
        return;
      }

      const btnDeleteDeadline = e.target.closest(".btn-delete-deadline");
      if (btnDeleteDeadline) {
        const jid = btnDeleteDeadline.getAttribute("data-jid");
        if (confirm(`Delete countdown deadline for group ${jid}?`)) {
          removeDeadline(jid);
        }
        return;
      }

      const btnRemoveTrack = e.target.closest(".btn-remove-party-track");
      if (btnRemoveTrack) {
        const idx = parseInt(btnRemoveTrack.getAttribute("data-index"), 10);
        if (!isNaN(idx)) {
          removePartyTrack(idx);
        }
        return;
      }
    });

    // MeowCaller & Song Party Event Handlers
    if (el.btnQuickOpenCaller) {
      el.btnQuickOpenCaller.addEventListener("click", () => switchTab("meowcaller"));
    }

    if (el.btnToggleCallerMaster) {
      el.btnToggleCallerMaster.addEventListener("click", toggleMeowCallerMaster);
    }

    if (el.callerDurationPresets) {
      el.callerDurationPresets.querySelectorAll(".duration-pill").forEach((pill) => {
        pill.addEventListener("click", () => {
          el.callerDurationPresets.querySelectorAll(".duration-pill").forEach((p) => p.classList.remove("active"));
          pill.classList.add("active");
          const mins = parseInt(pill.getAttribute("data-minutes") || "30", 10);
          state.selectedDurationMinutes = mins;
          if (state.meowcaller && state.meowcaller.enabled) {
            fetch("/api/meowcaller/toggle", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ enabled: true, durationMinutes: mins }),
            })
              .then((r) => r.json())
              .then((data) => {
                if (data && data.status) updateMeowCallerStatusUI(data.status);
                showToast(`Temporary window updated to ${mins > 0 ? mins + " mins" : "indefinite"}`);
              });
          }
        });
      });
    }

    if (el.checkCallerAutoAccept) {
      el.checkCallerAutoAccept.addEventListener("change", (e) => {
        toggleCallerAutoAccept(e.target.checked);
      });
    }

    if (el.btnCallerDial) {
      el.btnCallerDial.addEventListener("click", dialCaller);
    }

    if (el.btnCallerAnswer) {
      el.btnCallerAnswer.addEventListener("click", answerIncomingCaller);
    }

    if (el.btnCallerReject) {
      el.btnCallerReject.addEventListener("click", rejectIncomingCaller);
    }

    if (el.btnCallerHangup) {
      el.btnCallerHangup.addEventListener("click", hangupCaller);
    }

    if (el.btnPartyQueueSong) {
      el.btnPartyQueueSong.addEventListener("click", queuePartySongFromDashboard);
    }

    if (el.partySongInput) {
      el.partySongInput.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          queuePartySongFromDashboard();
        }
      });
    }

    if (el.btnPartyPauseResume) {
      el.btnPartyPauseResume.addEventListener("click", () => {
        if (state.meowcaller && state.meowcaller.isPaused) {
          controlParty("resume");
        } else {
          controlParty("pause");
        }
      });
    }

    if (el.btnPartySkip) {
      el.btnPartySkip.addEventListener("click", () => controlParty("skip"));
    }

    if (el.btnPartyStop) {
      el.btnPartyStop.addEventListener("click", () => controlParty("stop"));
    }

    // Radio Station DJ & Live Broadcast Mic Event Handlers
    if (el.btnRadioMicToggle) {
      el.btnRadioMicToggle.addEventListener("click", toggleRadioOnAir);
    }

    if (el.btnRadioPTT) {
      const startPTT = (e) => { e.preventDefault(); setRadioPTT(true); };
      const stopPTT = (e) => { e.preventDefault(); setRadioPTT(false); };
      el.btnRadioPTT.addEventListener("mousedown", startPTT);
      el.btnRadioPTT.addEventListener("mouseup", stopPTT);
      el.btnRadioPTT.addEventListener("mouseleave", () => { if (isRadioPttActive) setRadioPTT(false); });
      el.btnRadioPTT.addEventListener("touchstart", startPTT, { passive: false });
      el.btnRadioPTT.addEventListener("touchend", stopPTT, { passive: false });
      el.btnRadioPTT.addEventListener("touchcancel", stopPTT, { passive: false });
    }

    // Spacebar Push-to-Talk Hotkey
    window.addEventListener("keydown", (e) => {
      if (e.code === "Space" && state.currentTab === "meowcaller") {
        const tag = document.activeElement ? document.activeElement.tagName.toLowerCase() : "";
        if (tag === "input" || tag === "textarea" || tag === "select") return;
        e.preventDefault();
        setRadioPTT(true);
      }
    });

    window.addEventListener("keyup", (e) => {
      if (e.code === "Space" && isRadioPttActive) {
        e.preventDefault();
        setRadioPTT(false);
      }
    });

    if (el.radioModeSelector) {
      el.radioModeSelector.querySelectorAll(".duration-pill").forEach((pill) => {
        pill.addEventListener("click", () => {
          el.radioModeSelector.querySelectorAll(".duration-pill").forEach((p) => p.classList.remove("active"));
          pill.classList.add("active");
          const mode = pill.getAttribute("data-mode") || "ducking";
          radioMode = mode;
          sendRadioControl("set_mode", { mode });
          showToast(`DJ Mode set to: ${mode === "ducking" ? "Auto-Ducking" : "Mic Only (Mute Music)"}`, "info");
        });
      });
    }

    if (el.sliderRadioMicGain) {
      el.sliderRadioMicGain.addEventListener("input", (e) => {
        radioMicGain = parseFloat(e.target.value);
        if (el.labelRadioMicGain) el.labelRadioMicGain.textContent = `${Math.round(radioMicGain * 100)}%`;
        sendRadioControl("set_mic_gain", { micGain: radioMicGain });
      });
    }

    if (el.sliderRadioDuckLevel) {
      el.sliderRadioDuckLevel.addEventListener("input", (e) => {
        radioDuckLevel = parseFloat(e.target.value);
        if (el.labelRadioDuckLevel) el.labelRadioDuckLevel.textContent = `${Math.round(radioDuckLevel * 100)}%`;
        sendRadioControl("set_duck_level", { duckLevel: radioDuckLevel });
      });
    }

    if (el.selectRadioMicDevice) {
      el.selectRadioMicDevice.addEventListener("change", async () => {
        if (isRadioOnAir || isRadioPttActive) {
          stopRadioMicStream();
          await startRadioMicStream();
          showToast("Switched microphone hardware input", "success");
        }
      });
    }
  }

  // Initialization & Polling Cycle
  function startPolling() {
    // Initial fetch
    fetchStatus();
    fetchLogs();

    // Regular polling for status (every 2.5s)
    state.pollTimer = setInterval(() => {
      fetchStatus();
      if (state.currentTab === "logs") {
        fetchLogs();
      }
    }, 2500);
  }

  // Launch on DOM ready
  document.addEventListener("DOMContentLoaded", () => {
    initTheme();
    initModalListeners();
    initEventHandlers();
    populateRadioMicDevices();
    fetchVoiceBotVoices();
    fetchVoiceBotConfig();
    startPolling();
  });
})();
