(function () {
    "use strict";
    const $ = (s, r = document) => r.querySelector(s);
    const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));

    document.addEventListener("DOMContentLoaded", () => {
        const sidebar = $("#appSidebar");
        const navBtns = $$(".nav-btn");

        /* Connection state drives the header hairline */
        const dot = $("#footerStatusDot");
        const syncConn = () => {
            if (!dot) return;
            document.body.dataset.conn = dot.classList.contains("online") ? "online"
                : dot.classList.contains("warning") ? "warning" : "offline";
        };
        if (dot) { new MutationObserver(syncConn).observe(dot, { attributes: true, attributeFilter: ["class"] }); syncConn(); }

        /* Active tab: aria-current, document title, remembered across reloads */
        const base = "PrismBot Console";
        const syncTab = () => {
            const active = $(".nav-btn.active");
            if (!active) return;
            navBtns.forEach((b) => b === active ? b.setAttribute("aria-current", "page") : b.removeAttribute("aria-current"));
            const title = ($("#pageTitle") || {}).textContent || "";
            document.title = title ? `${title} | ${base}` : base;
            const tab = active.getAttribute("data-tab");
            try { localStorage.setItem("prism_tab", tab); } catch (e) { }
            if (history.replaceState) history.replaceState(null, "", "#" + tab);
        };
        navBtns.forEach((b) => new MutationObserver(syncTab).observe(b, { attributes: true, attributeFilter: ["class"] }));
        const wanted = location.hash.slice(1) || (function () { try { return localStorage.getItem("prism_tab"); } catch (e) { return null; } })();
        const target = wanted && navBtns.find((b) => b.getAttribute("data-tab") === wanted);
        if (target && wanted !== "overview") target.click(); else syncTab();

        /* Mobile drawer scrim, Escape to close */
        const scrim = document.createElement("div");
        scrim.className = "scrim";
        document.body.appendChild(scrim);
        if (sidebar) {
            new MutationObserver(() => scrim.classList.toggle("show", sidebar.classList.contains("open")))
                .observe(sidebar, { attributes: true, attributeFilter: ["class"] });
            scrim.addEventListener("click", () => sidebar.classList.remove("open"));
        }

        /* Accessibility: dialogs, live toasts, icon-only buttons, skip target */
        $$(".modal-overlay").forEach((m) => {
            m.setAttribute("role", "dialog");
            m.setAttribute("aria-modal", "true");
            const t = $(".modal-title", m);
            if (t) { t.id = t.id || "mt-" + m.id; m.setAttribute("aria-labelledby", t.id); }
            new MutationObserver(() => {
                if (m.classList.contains("active")) { const f = $("input,textarea,select", m); if (f) setTimeout(() => f.focus(), 30); }
            }).observe(m, { attributes: true, attributeFilter: ["class"] });
            /* Enter submits single-line modal fields */
            m.addEventListener("keydown", (e) => {
                if (e.key === "Enter" && e.target.tagName === "INPUT") {
                    const go = $(".modal-footer .btn-primary, .modal-footer .btn-danger", m);
                    if (go) { e.preventDefault(); go.click(); }
                }
            });
        });
        $$(".modal-close").forEach((b) => b.setAttribute("aria-label", "Close dialog"));
        const toasts = $("#toastContainer");
        if (toasts) { toasts.setAttribute("role", "status"); toasts.setAttribute("aria-live", "polite"); }
        $$("button[title]").forEach((b) => { if (!b.getAttribute("aria-label") && !b.textContent.trim()) b.setAttribute("aria-label", b.title); });
        const content = $(".app-content");
        if (content) { content.id = content.id || "mainContent"; content.setAttribute("tabindex", "-1"); }

        /* Command palette (Ctrl/Cmd+K) */
        const icon = (id) => `<svg><use href="#icon-${id}"/></svg>`;
        const items = navBtns.map((b) => {
            const use = $("use", b);
            return {
                label: b.textContent.replace(/\s+/g, " ").replace(/\b(OFF|LIVE|\d+m)$/, "").trim(),
                hint: "Go to tab", icon: use ? use.getAttribute("href").slice(6) : "cpu",
                run: () => b.click(),
            };
        });
        [["Send a message", "send", "#btnOpenSendMessageModal"], ["Sleep or wake the bot", "moon", "#btnToggleSleep"],
        ["Switch light or dark theme", "sun", "#btnToggleTheme"], ["Refresh status", "refresh", "#btnQuickRefresh"]]
            .forEach(([label, ic, sel]) => { const t = $(sel); if (t) items.push({ label, hint: "Action", icon: ic, run: () => t.click() }); });

        const overlay = document.createElement("div");
        overlay.className = "cmdk-overlay";
        overlay.innerHTML = '<div class="cmdk" role="dialog" aria-modal="true" aria-label="Jump to"><input type="text" placeholder="Jump to a tab or run an action" aria-label="Search commands" autocomplete="off" spellcheck="false"><ul class="cmdk-list" role="listbox"></ul></div>';
        document.body.appendChild(overlay);
        const input = $("input", overlay), list = $(".cmdk-list", overlay);
        let shown = [], sel = 0;

        const draw = () => {
            const q = input.value.trim().toLowerCase();
            shown = items.filter((i) => i.label.toLowerCase().includes(q));
            sel = Math.min(sel, Math.max(shown.length - 1, 0));
            list.innerHTML = shown.length
                ? shown.map((i, n) => `<li class="cmdk-item" role="option" data-n="${n}" aria-selected="${n === sel}">${icon(i.icon)}<span>${i.label}</span><small>${i.hint}</small></li>`).join("")
                : '<li class="cmdk-empty">Nothing matches that search.</li>';
            const cur = $('[aria-selected="true"]', list); if (cur) cur.scrollIntoView({ block: "nearest" });
        };
        const open = () => { overlay.classList.add("open"); input.value = ""; sel = 0; draw(); input.focus(); };
        const close = () => overlay.classList.remove("open");
        const run = (n) => { const i = shown[n]; if (i) { close(); i.run(); } };

        input.addEventListener("input", () => { sel = 0; draw(); });
        input.addEventListener("keydown", (e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); sel = (sel + 1) % Math.max(shown.length, 1); draw(); }
            else if (e.key === "ArrowUp") { e.preventDefault(); sel = (sel - 1 + shown.length) % Math.max(shown.length, 1); draw(); }
            else if (e.key === "Enter") { e.preventDefault(); run(sel); }
            else if (e.key === "Escape") { e.stopPropagation(); close(); }
        });
        list.addEventListener("click", (e) => { const li = e.target.closest(".cmdk-item"); if (li) run(+li.dataset.n); });
        overlay.addEventListener("mousedown", (e) => { if (e.target === overlay) close(); });
        window.addEventListener("keydown", (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") { e.preventDefault(); overlay.classList.contains("open") ? close() : open(); }
            else if (e.key === "Escape" && sidebar) sidebar.classList.remove("open");
        });

        const actions = $(".header-actions");
        if (actions) {
            const trigger = document.createElement("button");
            trigger.className = "btn btn-sm cmdk-trigger";
            trigger.setAttribute("aria-label", "Open command palette");
            trigger.innerHTML = `${icon("sliders")}<span>Jump to</span><span class="kbd">Ctrl K</span>`;
            if (/Mac|iPhone|iPad/.test(navigator.platform)) $(".kbd", trigger).textContent = "\u2318K";
            trigger.addEventListener("click", open);
            actions.insertBefore(trigger, actions.firstChild);
        }
    });
})();