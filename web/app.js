// AnimeUpscale Web UI — vanilla JS
(() => {
  "use strict";

  // ---------- elements ----------
  const $ = (id) => document.getElementById(id);
  const form = $("upscale-form");
  const fileInput = $("file-input");
  const dropzone = $("dropzone");
  const dropzoneName = $("dropzone-name");
  const engineGrid = $("engine-grid");
  const engineStatus = $("engine-status");
  const enginesBadge = $("engines-badge");
  const modelName = $("model-name");
  const modelHint = $("model-hint");
  const modeInputs = document.querySelectorAll('input[name="mode"]');
  const modeHint = $("mode-hint");
  const btnRun = $("btn-run");
  const btnReset = $("btn-reset");
  const btnCancel = $("btn-cancel");
  const btnDownload = $("btn-download");
  const btnClearLog = $("btn-clear-log");
  const btnCopyCmd = $("btn-copy-cmd");
  const statusDot = $("status-dot");
  const statusText = $("status-text");
  const logEl = $("log");
  const cmdPreview = $("cmd-preview");
  const cmdText = $("cmd-text");
  const previewInputBody = $("preview-input-body");
  const previewOutputBody = $("preview-output-body");
  const toasts = $("toasts");

  const noiseInput = $("noise");
  const noiseOut = $("noise-out");
  const sharpenInput = $("sharpen");
  const sharpenOut = $("sharpen-out");
  const scaleSelect = $("scale");
  const targetSelect = $("target");
  const tileSize = $("tile-size");
  const gpuInput = $("gpu");

  // ---------- state ----------
  let pickedFile = null;
  let pickedURL = null;
  let engines = [];
  let currentJobId = null;
  let currentJobOutputURL = null;
  let currentJobOutputName = null;
  let currentJobIsVideo = false;
  let currentCmdLine = "";
  let es = null;

  // ---------- engine metadata ----------
  // First model in each list is the recommended default for that engine.
  const ENGINE_MODELS = {
    auto: [{ value: "", label: "Auto (use engine default)" }],
    realesrgan: [
      { value: "realesr-animevideov3", label: "realesr-animevideov3 — anime/video, recommended" },
      { value: "realesrgan-x4plus-anime", label: "realesrgan-x4plus-anime — anime illustrations" },
      { value: "realesrgan-x4plus", label: "realesrgan-x4plus — general/real-world" },
      { value: "realesr-general-x4v3", label: "realesr-general-x4v3 — general v3" },
    ],
    realsr: [
      { value: "DF2K_JPEG", label: "DF2K_JPEG — denoised (for compressed/JPEG input)" },
      { value: "DF2K", label: "DF2K — clean (for high-quality photos)" },
    ],
    builtin: [{ value: "", label: "Built-in has no model selection" }],
    waifu2x: [
      { value: "models-cunet", label: "models-cunet — anime style" },
      { value: "models-upconv_7_anime_style_art_rgb", label: "models-upconv_7_anime_style_art_rgb" },
      { value: "models-upconv_7_photo", label: "models-upconv_7_photo — photographic" },
    ],
    realcugan: [
      { value: "models-se", label: "models-se — conservative, fewer artifacts" },
      { value: "models-pro", label: "models-pro — balanced" },
      { value: "models-nose", label: "models-nose — aggressive" },
    ],
    anime4kcpp: [
      { value: "acnet-legacy-gan", label: "acnet-legacy-gan — default, high performance" },
      { value: "acnet-legacy-hdn0", label: "acnet-legacy-hdn0 — light denoise" },
      { value: "acnet-legacy-hdn1", label: "acnet-legacy-hdn1" },
      { value: "acnet-legacy-hdn2", label: "acnet-legacy-hdn2" },
      { value: "acnet-legacy-hdn3", label: "acnet-legacy-hdn3 — heavy denoise" },
      { value: "acnet-f8b18-box", label: "acnet-f8b18-box — vector / sharp line-art" },
      { value: "acnet-f8b8", label: "acnet-f8b8" },
      { value: "acnet-f8b16", label: "acnet-f8b16" },
      { value: "arnet-f8b8", label: "arnet-f8b8" },
      { value: "artcnn-c4f32", label: "artcnn-c4f32 — premium neutral" },
      { value: "artcnn-c4f16", label: "artcnn-c4f16" },
      { value: "artcnn-c4f32-dn", label: "artcnn-c4f32-dn — denoise + soften" },
      { value: "artcnn-c4f32-ds", label: "artcnn-c4f32-ds — denoise + sharpen" },
      { value: "fsrcnnx-f8b4", label: "fsrcnnx-f8b4" },
      { value: "fsrcnnx-f16b4", label: "fsrcnnx-f16b4" },
      { value: "fsrcnnx-f16b4-distort-plus", label: "fsrcnnx-f16b4-distort-plus — heavy artifact recovery" },
    ],
  };

  const ENGINE_DEFAULT_GPU = {
    realesrgan: "auto",
    realsr: "auto",
    builtin: "auto",
    waifu2x: "auto",
    realcugan: "auto",
    anime4kcpp: "opencl",
  };

  // ---------- toast helper ----------
  function toast(message, kind = "ok", timeout = 3500) {
    const el = document.createElement("div");
    el.className = "toast " + kind;
    el.textContent = message;
    toasts.appendChild(el);
    setTimeout(() => {
      el.style.transition = "opacity .25s";
      el.style.opacity = "0";
      setTimeout(() => el.remove(), 280);
    }, timeout);
  }

  // ---------- log helpers ----------
  function appendLog(line, cls = "") {
    const span = document.createElement("span");
    span.className = "line" + (cls ? " " + cls : "");
    span.textContent = line + "\n";
    logEl.appendChild(span);
    logEl.scrollTop = logEl.scrollHeight;
  }
  function clearLog() {
    logEl.textContent = "";
  }
  function classifyLogLine(line) {
    if (line.startsWith("[err]")) return "err";
    if (line.startsWith("[server]")) return "server";
    if (line.startsWith("stage:") || line.startsWith("engine:") || line.startsWith("output:") || line.startsWith("size:")) return "status";
    return "";
  }

  // ---------- engine population ----------
  async function loadEngines() {
    try {
      const r = await fetch("/api/engines");
      if (!r.ok) throw new Error("engines fetch " + r.status);
      const data = await r.json();
      engines = data.engines || [];
      renderEngines();
      const avail = engines.filter((e) => e.available).length;
      enginesBadge.classList.remove("muted");
      enginesBadge.innerHTML = `<span class="pulse"></span>${avail}/${engines.length} engines ready`;
    } catch (err) {
      enginesBadge.textContent = "engines offline";
      enginesBadge.classList.add("muted");
      console.error(err);
    }
  }

  function renderEngines() {
    engineGrid.innerHTML = "";
    const order = ["auto", "realesrgan", "realsr", "builtin", "anime4kcpp", "waifu2x", "realcugan"];
    const byName = new Map(engines.map((e) => [e.name, e]));
    const items = order
      .map((name) => byName.get(name) || { name, available: false, note: "unknown" })
      .concat(engines.filter((e) => !order.includes(e.name)));

    for (const e of items) {
      const label = document.createElement("label");
      label.className = "engine-card";
      if (!e.available && e.name !== "auto") label.classList.add("disabled");
      label.dataset.engine = e.name;
      label.innerHTML = `
        <input type="radio" name="engine" value="${e.name}" ${e.name === "auto" ? "checked" : ""} ${!e.available && e.name !== "auto" ? "disabled" : ""}/>
        <span class="name">${e.name}</span>
        <span class="note">${escapeHTML(e.note || "")}</span>
        <span class="pill ${e.available ? "ok" : "miss"}">${e.available ? "ready" : "missing"}</span>
      `;
      engineGrid.appendChild(label);
    }

    // event delegation
    engineGrid.addEventListener("change", (ev) => {
      if (ev.target && ev.target.name === "engine") {
        onEngineChange(ev.target.value);
      }
    });
    onEngineChange("auto");
  }

  function onEngineChange(engine) {
    updateModelSuggestions(engine);
    showEngineSpecificFields(engine, currentMode());
    showVideoFields(currentMode() === "video");
    // update visual selected state
    engineGrid.querySelectorAll(".engine-card").forEach((card) => {
      const checked = card.querySelector("input").checked;
      card.classList.toggle("checked", checked);
    });
    // if the chosen engine is missing and not "auto", warn
    const info = engines.find((e) => e.name === engine);
    if (info && !info.available) {
      engineStatus.textContent = "this engine isn't installed locally";
    } else {
      engineStatus.textContent = "";
    }
    if (ENGINE_DEFAULT_GPU[engine] && !gpuInput.value) {
      gpuInput.value = ENGINE_DEFAULT_GPU[engine];
    }
  }

  function updateModelSuggestions(engine) {
    modelList.innerHTML = "";
    const list = ENGINE_MODELS[engine] || [];
    for (const m of list) {
      const opt = document.createElement("option");
      opt.value = m;
      modelList.appendChild(opt);
    }
    modelHint.textContent = list.length
      ? `Suggestions: ${list.slice(0, 4).join(", ")}${list.length > 4 ? "…" : ""}. Leave blank for the default.`
      : "Leave blank to use the engine default.";
  }

  function showEngineSpecificFields(engine, mode) {
    // engine-specific collapsibles
    document.querySelectorAll(".field.collapsible[data-engine]").forEach((el) => {
      const allowed = (el.dataset.engine || "").split(",").map((s) => s.trim());
      el.classList.toggle("visible", allowed.includes(engine));
    });
  }

  function showVideoFields(isVideo) {
    document.querySelectorAll(".field.collapsible[data-mode]").forEach((el) => {
      const allowed = (el.dataset.mode || "").split(",").map((s) => s.trim());
      el.classList.toggle("visible", allowed.includes(isVideo ? "video" : "image"));
    });
  }

  // ---------- mode ----------
  function currentMode() {
    const v = document.querySelector('input[name="mode"]:checked')?.value || "auto";
    if (v !== "auto") return v;
    if (!pickedFile) return "image";
    const name = pickedFile.name.toLowerCase();
    if (/\.(mp4|mkv|mov|avi|webm)$/.test(name)) return "video";
    return "image";
  }

  modeInputs.forEach((r) => r.addEventListener("change", () => {
    const m = currentMode();
    modeHint.textContent = m === "video"
      ? "Video mode: frames are extracted, upscaled in parallel, then muxed with audio."
      : "Image mode: single image, upscaled in one pass.";
    showVideoFields(m === "video");
  }));

  // ---------- file picking ----------
  function setFile(file) {
    if (!file) return;
    if (pickedURL) URL.revokeObjectURL(pickedURL);
    pickedFile = file;
    pickedURL = URL.createObjectURL(file);
    dropzone.classList.add("has-file");
    dropzoneName.hidden = false;
    dropzoneName.textContent = `${file.name} · ${formatBytes(file.size)}`;
    showInputPreview(file, pickedURL);
    const m = currentMode();
    showVideoFields(m === "video");
    if (m === "video" && document.querySelector('input[name="engine"]:checked')?.value === "builtin") {
      // builtin can't do video; bump to auto
      const auto = document.querySelector('input[name="engine"][value="auto"]');
      if (auto) { auto.checked = true; onEngineChange("auto"); }
      toast("Built-in engine doesn't support video — switched to auto.", "warn");
    }
  }

  function showInputPreview(file, url) {
    previewInputBody.innerHTML = "";
    if (file.type.startsWith("image/")) {
      const img = document.createElement("img");
      img.src = url;
      img.alt = "input preview";
      previewInputBody.appendChild(img);
    } else if (file.type.startsWith("video/")) {
      const v = document.createElement("video");
      v.src = url;
      v.controls = true;
      v.preload = "metadata";
      previewInputBody.appendChild(v);
    } else {
      const p = document.createElement("p");
      p.className = "placeholder";
      p.textContent = `${file.name} (${file.type || "unknown"})`;
      previewInputBody.appendChild(p);
    }
  }

  // ---------- dropzone events ----------
  // The dropzone is a <label for="file-input">, so a native click on it
  // opens the file picker. We just need to handle change, drag/drop, and
  // keyboard activation (Enter/Space when the label is focused).
  fileInput.addEventListener("change", () => {
    const f = fileInput.files && fileInput.files[0];
    if (f) setFile(f);
  });

  ["dragenter", "dragover"].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => {
      e.preventDefault(); e.stopPropagation();
      dropzone.classList.add("dragover");
    });
  });
  ["dragleave", "dragend"].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => {
      e.preventDefault(); e.stopPropagation();
      dropzone.classList.remove("dragover");
    });
  });
  dropzone.addEventListener("drop", (e) => {
    e.preventDefault(); e.stopPropagation();
    dropzone.classList.remove("dragover");
    const f = e.dataTransfer?.files?.[0];
    if (f) setFile(f);
  });

  // Keyboard activation: Enter or Space on the focused label opens the picker.
  dropzone.addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      fileInput.click();
    }
  });
  });
  ["dragleave", "drop"].forEach((evt) => {
    dropzone.addEventListener("dragover", (e) => e.preventDefault());
    dropzone.addEventListener("drop", (e) => {
      e.preventDefault();
      dropzone.classList.remove("dragover");
      const f = e.dataTransfer?.files?.[0];
      if (f) setFile(f);
    });
  });

  // ---------- sliders ----------
  noiseInput.addEventListener("input", () => {
    const v = parseInt(noiseInput.value, 10);
    noiseOut.textContent = v === -1 ? "auto" : String(v);
  });
  sharpenInput.addEventListener("input", () => {
    sharpenOut.textContent = parseFloat(sharpenInput.value).toFixed(2);
  });

  // ---------- form submission ----------
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (!pickedFile) { toast("Pick a file first.", "warn"); return; }
    await startJob();
  });

  btnReset.addEventListener("click", resetAll);

  function resetAll() {
    form.reset();
    if (pickedURL) { URL.revokeObjectURL(pickedURL); pickedURL = null; }
    pickedFile = null;
    dropzone.classList.remove("has-file");
    dropzoneName.hidden = true;
    previewInputBody.innerHTML = '<p class="muted small">No file yet.</p>';
    previewOutputBody.innerHTML = '<p class="muted small">Result will appear here.</p>';
    clearLog();
    setStatus("idle", "Idle");
    btnDownload.hidden = true;
    if (currentJobOutputURL) { URL.revokeObjectURL(currentJobOutputURL); currentJobOutputURL = null; }
    currentJobId = null;
    onEngineChange("auto");
    modeInputs.forEach((r) => { if (r.value === "auto") r.checked = true; });
    showVideoFields(false);
    noiseOut.textContent = "0";
    sharpenOut.textContent = "0.15";
    cmdPreview.hidden = true;
  }

  // ---------- start job ----------
  async function startJob() {
    btnRun.disabled = true;
    btnRun.querySelector(".btn-label").textContent = "Running…";
    btnRun.querySelector(".btn-spinner").hidden = false;
    btnCancel.hidden = false;
    btnDownload.hidden = true;
    previewOutputBody.innerHTML = '<p class="muted small">Working…</p>';
    setStatus("running", "Starting…");
    clearLog();

    const fd = new FormData();
    fd.append("file", pickedFile, pickedFile.name);
    const formData = new FormData(form);
    for (const [k, v] of formData.entries()) {
      if (k === "file") continue;
      if (typeof v === "string") fd.append(k, v);
    }
    // ensure engine is included even if radios are wonky
    const engine = document.querySelector('input[name="engine"]:checked')?.value || "auto";
    fd.set("engine", engine);

    try {
      const r = await fetch("/api/upscale", { method: "POST", body: fd });
      if (!r.ok) {
        const txt = await r.text();
        throw new Error(txt || `upload failed (${r.status})`);
      }
      const data = await r.json();
      currentJobId = data.id;
      currentJobOutputName = data.outputName;
      currentJobIsVideo = data.isVideo;
      currentCmdLine = data.cmdLine || "";
      cmdText.textContent = currentCmdLine;
      cmdPreview.hidden = false;
      appendLog("[server] " + currentCmdLine, "server");
      openSSE(currentJobId);
    } catch (err) {
      finishWithError(err.message || String(err));
    }
  }

  // ---------- SSE stream ----------
  function openSSE(jobId) {
    if (es) { es.close(); es = null; }
    es = new EventSource(`/api/jobs/${jobId}/events`);
    es.addEventListener("log", (ev) => {
      try {
        const data = JSON.parse(ev.data);
        appendLog(data.message, classifyLogLine(data.message));
      } catch {}
    });
    es.addEventListener("status", (ev) => {
      try {
        const data = JSON.parse(ev.data);
        const map = { queued: "queued", running: "running", done: "done", error: "error" };
        const label = {
          queued: "Queued",
          running: "Running…",
          done: "Done",
          error: "Error",
        }[data.status] || data.status;
        setStatus(map[data.status] || "idle", label);
        if (data.status === "done") finishJob(true, "");
        else if (data.status === "error") finishJob(false, data.message || "job failed");
      } catch {}
    });
    es.addEventListener("ping", () => {});
    es.addEventListener("end", () => { if (es) { es.close(); es = null; } });
    es.onerror = () => {
      // browser auto-reconnects; we let the final status message close it cleanly
    };
  }

  function finishJob(ok, errMsg) {
    btnRun.disabled = false;
    btnRun.querySelector(".btn-label").textContent = "Upscale";
    btnRun.querySelector(".btn-spinner").hidden = true;
    btnCancel.hidden = true;

    if (ok) {
      toast("Upscale complete.", "ok");
      const url = `/api/output/${currentJobId}?t=${Date.now()}`;
      showOutput(url);
      btnDownload.hidden = false;
      btnDownload.onclick = () => triggerDownload(url, currentJobOutputName || "output");
      setStatus("done", "Done");
    } else {
      finishWithError(errMsg);
    }
    if (es) { es.close(); es = null; }
  }

  function finishWithError(msg) {
    setStatus("error", "Error");
    appendLog("[error] " + msg, "err");
    toast(msg, "err", 5000);
    btnRun.disabled = false;
    btnRun.querySelector(".btn-label").textContent = "Upscale";
    btnRun.querySelector(".btn-spinner").hidden = true;
    btnCancel.hidden = true;
    previewOutputBody.innerHTML = `<p class="placeholder" style="color: var(--danger)">${escapeHTML(msg)}</p>`;
  }

  function showOutput(url) {
    previewOutputBody.innerHTML = "";
    if (currentJobIsVideo) {
      const v = document.createElement("video");
      v.src = url;
      v.controls = true;
      v.preload = "metadata";
      previewOutputBody.appendChild(v);
    } else {
      const img = document.createElement("img");
      img.src = url;
      img.alt = "output preview";
      img.onload = () => { /* fine */ };
      img.onerror = () => {
        previewOutputBody.innerHTML = `<p class="placeholder">Output ready, but preview could not be loaded.<br/><a class="btn primary" href="${url}" download>Download</a></p>`;
      };
      previewOutputBody.appendChild(img);
    }
  }

  function triggerDownload(url, filename) {
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  btnCancel.addEventListener("click", () => {
    if (es) { es.close(); es = null; }
    finishWithError("Cancelled by user");
  });

  btnClearLog.addEventListener("click", clearLog);
  btnCopyCmd.addEventListener("click", async () => {
    if (!currentCmdLine) { toast("No command yet.", "warn"); return; }
    try {
      await navigator.clipboard.writeText(currentCmdLine);
      toast("Command copied to clipboard.", "ok", 1800);
    } catch {
      toast("Could not access clipboard.", "err");
    }
  });

  function setStatus(state, label) {
    statusDot.dataset.state = state;
    statusText.textContent = label;
  }

  // ---------- helpers ----------
  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
  }
  function formatBytes(n) {
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + " MB";
    return (n / 1024 / 1024 / 1024).toFixed(2) + " GB";
  }

  // ---------- init ----------
  loadEngines();
  onEngineChange("auto");
  setStatus("idle", "Idle");
})();
