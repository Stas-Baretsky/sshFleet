(function () {
  "use strict";

  /* ============================================================
   * DOM helpers
   * ============================================================ */

  function h(tag, attrs, children) {
    attrs = attrs || {};
    const e = document.createElement(tag);

    for (const key of Object.keys(attrs)) {
      const val = attrs[key];

      if (val === undefined || val === null || val === false) continue;

      if (key === "class") {
        e.className = val;
      } else if (key === "html") {
        e.innerHTML = val;
      } else if (key.indexOf("on") === 0 && typeof val === "function") {
        e.addEventListener(key.slice(2).toLowerCase(), val);
      } else if (key in e) {
        try {
          e[key] = val;
        } catch (_) {
          e.setAttribute(key, val === true ? "" : String(val));
        }
      } else {
        e.setAttribute(key, val === true ? "" : String(val));
      }
    }

    const kids = children === undefined ? [] : Array.isArray(children) ? children : [children];

    for (const c of kids) {
      if (c === null || c === undefined || c === false) continue;
      e.appendChild(typeof c === "string" || typeof c === "number" ? document.createTextNode(String(c)) : c);
    }

    return e;
  }

  function clear(el) {
    while (el.firstChild) el.removeChild(el.firstChild);
  }

  function mount(el, children) {
    clear(el);
    (Array.isArray(children) ? children : [children]).forEach((c) => {
      if (c) el.appendChild(c);
    });
  }

  function parseIntSafe(v, fallback) {
    const n = parseInt(v, 10);
    return Number.isFinite(n) ? n : fallback;
  }

  function parseFloatSafe(v, fallback) {
    const n = parseFloat(v);
    return Number.isFinite(n) ? n : fallback;
  }

  function formatDateTime(iso) {
    if (!iso) return "";
    try {
      const d = new Date(iso);
      return d.toLocaleString("ru-RU", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      });
    } catch (_) {
      return iso;
    }
  }

  function errMsg(err) {
    if (!err) return "Неизвестная ошибка";
    if (typeof err === "string") return err;
    if (err.message) return err.message;
    try {
      return JSON.stringify(err);
    } catch (_) {
      return String(err);
    }
  }

  /* ============================================================
   * Backend API (window.go.<package>.<Struct>.<Method> - внедряется Wails)
   * ============================================================ */

  const api = {
    getGroups: () => window.go.main.App.GetGroups(),
    saveGroup: (g) => window.go.main.App.SaveGroup(g),
    deleteGroup: (id) => window.go.main.App.DeleteGroup(id),
    getConnectionSettings: () => window.go.main.App.GetConnectionSettings(),
    saveConnectionSettings: (s) => window.go.main.App.SaveConnectionSettings(s),
    pickPrivateKeyFile: () => window.go.main.App.PickPrivateKeyFile(),
    pickKnownHostsFile: () => window.go.main.App.PickKnownHostsFile(),
    startRun: (req) => window.go.main.App.StartRun(req),
    cancelRun: (id) => window.go.main.App.CancelRun(id),
    listHistory: () => window.go.main.App.ListHistory(),
    getRun: (id) => window.go.main.App.GetRun(id),
    getDeviceOutput: (runId, deviceId) => window.go.main.App.GetDeviceOutput(runId, deviceId),
  };

  /* ============================================================
   * State
   * ============================================================ */

  const state = {
    tab: "devices",
    devices: [{ name: "", address: "" }],
    groups: [],
    description: "",
    commandsText: "",
    settings: null,
    currentRun: null,
    historyList: [],
    historyDetail: null,
    starting: false,
    cancelling: false,
  };

  /* ============================================================
   * Toasts
   * ============================================================ */

  function toast(message, kind) {
    const root = document.getElementById("toast-root");
    const t = h("div", { class: "toast" + (kind ? " " + kind : "") }, message);
    root.appendChild(t);
    setTimeout(() => t.remove(), 4000);
  }

  /* ============================================================
   * Modal
   * ============================================================ */

  function openModal(opts) {
    const root = document.getElementById("modal-root");
    clear(root);

    const close = () => clear(root);

    const dialog = h("div", { class: "modal" + (opts.wide ? " modal-wide" : "") }, [
      h("div", { class: "modal-header" }, [
        h("div", { class: "modal-title" }, opts.title),
        h("button", { class: "icon-btn", onClick: close }, "✕"),
      ]),
      h("div", { class: "modal-body" }, opts.body),
      opts.footer ? h("div", { class: "modal-footer" }, opts.footer) : null,
    ]);

    const backdrop = h(
      "div",
      {
        class: "modal-backdrop",
        onClick: (e) => {
          if (e.target === backdrop) close();
        },
      },
      [dialog]
    );

    root.appendChild(backdrop);

    return close;
  }

  /* ============================================================
   * Navigation
   * ============================================================ */

  function setTab(tab) {
    state.tab = tab;

    document.querySelectorAll(".nav-item").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.tab === tab);
    });

    document.querySelectorAll(".tab").forEach((sec) => {
      sec.classList.toggle("active", sec.id === "tab-" + tab);
    });

    renderActiveTab();
  }

  function renderActiveTab() {
    switch (state.tab) {
      case "devices":
        renderDevices();
        break;
      case "commands":
        renderCommands();
        break;
      case "output":
        renderOutput();
        break;
      case "history":
        renderHistory();
        break;
      case "settings":
        renderSettings();
        break;
    }
  }

  function updateNavBadge() {
    const badge = document.getElementById("nav-badge-output");
    if (!badge) return;

    if (state.currentRun && !state.currentRun.finishedAt) {
      const pending = state.currentRun.devices.filter(
        (d) => d.status === "running" || d.status === "pending"
      ).length;
      badge.hidden = false;
      badge.textContent = String(pending);
    } else {
      badge.hidden = true;
    }
  }

  /* ============================================================
   * Devices tab
   * ============================================================ */

  function renderDevices() {
    const root = document.getElementById("tab-devices");

    const rowsContainer = h("div", { id: "device-rows" });
    renderDeviceRows(rowsContainer);

    const groupOptions = [h("option", { value: "" }, "Выбрать группу…")].concat(
      state.groups.map((g) => h("option", { value: g.id }, g.name + " (" + g.devices.length + ")"))
    );

    const groupSelect = h("select", { class: "group-select" }, groupOptions);

    mount(root, [
      h("div", { class: "page-header" }, [
        h("div", {}, [
          h("h1", { class: "page-title" }, "Устройства"),
          h(
            "p",
            { class: "page-subtitle" },
            "Укажите адрес каждого устройства. Имя не обязательно — если его не задать, в выводе и истории будет использован адрес."
          ),
        ]),
      ]),
      h("div", { class: "card" }, [
        h("div", { class: "toolbar" }, [
          groupSelect,
          h("button", { class: "btn btn-secondary", onClick: () => loadGroup(groupSelect.value) }, "Загрузить"),
          h("div", { class: "toolbar-spacer" }),
          state.groups.length
            ? h("button", { class: "btn btn-ghost", onClick: openManageGroupsModal }, "Управление группами")
            : null,
          h("button", { class: "btn btn-secondary", onClick: openSaveGroupModal }, "Сохранить как группу"),
        ]),
        h("div", { class: "divider" }),
        rowsContainer,
        h("button", { class: "btn btn-secondary add-device-btn", onClick: addDeviceRow }, "+ Добавить устройство"),
      ]),
    ]);
  }

  function renderDeviceRows(container) {
    clear(container);

    state.devices.forEach((d, i) => {
      const addrInput = h("input", {
        type: "text",
        class: "addr-input",
        placeholder: "IP или хост, например 10.0.0.1",
        value: d.address,
        onInput: (e) => {
          state.devices[i].address = e.target.value;
        },
      });

      const nameInput = h("input", {
        type: "text",
        class: "name-input",
        placeholder: "Имя (необязательно)",
        value: d.name,
        onInput: (e) => {
          state.devices[i].name = e.target.value;
        },
      });

      const removeBtn = h(
        "button",
        {
          class: "icon-btn",
          title: "Удалить",
          onClick: () => {
            state.devices.splice(i, 1);
            if (state.devices.length === 0) state.devices.push({ name: "", address: "" });
            renderDeviceRows(container);
          },
        },
        "✕"
      );

      container.appendChild(h("div", { class: "device-row" }, [addrInput, nameInput, removeBtn]));
    });
  }

  function addDeviceRow() {
    state.devices.push({ name: "", address: "" });
    renderDeviceRows(document.getElementById("device-rows"));
  }

  function loadGroup(groupId) {
    if (!groupId) return;

    const g = state.groups.find((x) => x.id === groupId);
    if (!g) return;

    state.devices = g.devices.map((d) => ({ name: d.name, address: d.address }));
    renderDevices();
    toast("Группа «" + g.name + "» загружена (" + g.devices.length + " устройств)");
  }

  function openSaveGroupModal() {
    const validDevices = state.devices.filter((d) => d.address.trim() !== "");

    if (validDevices.length === 0) {
      toast("Сначала укажите хотя бы один адрес устройства", "error");
      return;
    }

    const nameInput = h("input", { type: "text", placeholder: "Например: Ядро сети" });

    const close = openModal({
      title: "Сохранить как группу",
      body: h("div", { class: "field" }, [
        h("label", {}, "Имя группы"),
        nameInput,
        h("p", { class: "hint" }, validDevices.length + " устройств будет сохранено."),
      ]),
      footer: [
        h("button", { class: "btn btn-secondary", onClick: () => close() }, "Отмена"),
        h(
          "button",
          {
            class: "btn btn-primary",
            onClick: async () => {
              const name = nameInput.value.trim();

              if (!name) {
                toast("Введите имя группы", "error");
                return;
              }

              try {
                await api.saveGroup({
                  id: "",
                  name,
                  devices: validDevices.map((d) => ({ name: d.name.trim(), address: d.address.trim() })),
                });
                toast("Группа сохранена", "success");
                close();
                await loadGroups();
                if (state.tab === "devices") renderDevices();
              } catch (e) {
                toast(errMsg(e), "error");
              }
            },
          },
          "Сохранить"
        ),
      ],
    });

    nameInput.focus();
  }

  function openManageGroupsModal() {
    const rows = state.groups.map((g) =>
      h("div", { class: "history-row", style: "cursor: default;" }, [
        h("div", {}, [
          h("div", { class: "history-row-title" }, g.name),
          h("div", { class: "history-row-meta" }, g.devices.length + " устройств"),
        ]),
        h(
          "button",
          {
            class: "icon-btn",
            title: "Удалить группу",
            onClick: async () => {
              try {
                await api.deleteGroup(g.id);
                await loadGroups();
                openManageGroupsModal();
                if (state.tab === "devices") renderDevices();
              } catch (e) {
                toast(errMsg(e), "error");
              }
            },
          },
          "✕"
        ),
      ])
    );

    openModal({
      title: "Группы устройств",
      body: h("div", { class: "history-list" }, rows.length ? rows : [h("div", { class: "hint" }, "Групп пока нет")]),
    });
  }

  /* ============================================================
   * Commands tab
   * ============================================================ */

  function renderCommands() {
    const root = document.getElementById("tab-commands");

    const running = !!(state.currentRun && !state.currentRun.finishedAt);

    const descInput = h("input", {
      type: "text",
      placeholder: "Например: Смена пароля admin на всех коммутаторах",
      value: state.description,
      onInput: (e) => {
        state.description = e.target.value;
      },
    });

    const cmdArea = h("textarea", {
      class: "mono",
      rows: 12,
      placeholder:
        "По одной команде на строку, например:\nconfigure terminal\nusername admin password NewPass123\nexit\nwrite",
      value: state.commandsText,
      onInput: (e) => {
        state.commandsText = e.target.value;
      },
    });

    const runBtn = h(
      "button",
      {
        class: "btn btn-primary",
        disabled: state.starting || running,
        onClick: startRun,
      },
      state.starting ? "Запуск…" : "▶ Выполнить"
    );

    const cancelBtn = running
      ? h(
          "button",
          {
            class: "btn btn-danger",
            disabled: state.cancelling,
            onClick: cancelCurrentRun,
          },
          state.cancelling ? "Останавливаем…" : "Остановить"
        )
      : null;

    mount(root, [
      h("div", { class: "page-header" }, [
        h("div", {}, [
          h("h1", { class: "page-title" }, "Команды"),
          h(
            "p",
            { class: "page-subtitle" },
            "Команды выполняются по очереди на каждом устройстве из списка «Устройства». Чтобы входить в режим настройки (configure terminal и т.п.), включите интерактивный режим в Настройках."
          ),
        ]),
      ]),
      h("div", { class: "card" }, [
        h("div", { class: "field" }, [h("label", {}, "Описание запуска"), descInput]),
        h("div", { class: "field" }, [h("label", {}, "Команды"), cmdArea]),
        h("div", { class: "toolbar" }, [
          runBtn,
          cancelBtn,
          running ? h("span", { class: "hint" }, "Выполняется — статус смотрите на вкладке «Вывод»") : null,
        ]),
      ]),
    ]);
  }

  async function startRun() {
    const devices = state.devices
      .filter((d) => d.address.trim() !== "")
      .map((d) => ({ name: d.name.trim(), address: d.address.trim() }));

    if (devices.length === 0) {
      toast("Добавьте хотя бы одно устройство на вкладке «Устройства»", "error");
      setTab("devices");
      return;
    }

    const commands = state.commandsText
      .split("\n")
      .map((c) => c.trim())
      .filter((c) => c.length > 0);

    if (commands.length === 0) {
      toast("Добавьте хотя бы одну команду", "error");
      return;
    }

    state.starting = true;
    renderCommands();

    try {
      const meta = await api.startRun({ description: state.description.trim(), devices, commands });
      state.currentRun = meta;
      updateNavBadge();
      toast("Запуск начат: " + devices.length + " устройств", "success");
      setTab("output");
    } catch (e) {
      toast(errMsg(e), "error");
    } finally {
      state.starting = false;
      if (state.tab === "commands") renderCommands();
    }
  }

  async function cancelCurrentRun() {
    if (!state.currentRun) return;

    state.cancelling = true;
    renderActiveTab();

    try {
      await api.cancelRun(state.currentRun.id);
    } catch (e) {
      toast(errMsg(e), "error");
    } finally {
      state.cancelling = false;
    }
  }

  /* ============================================================
   * Output tab / shared run-detail renderer
   * ============================================================ */

  function renderOutput() {
    const root = document.getElementById("tab-output");

    if (!state.currentRun) {
      mount(root, [
        h("div", { class: "page-header" }, [h("div", {}, [h("h1", { class: "page-title" }, "Вывод")])]),
        h("div", { class: "card" }, [
          h("div", { class: "empty-state" }, [
            h("div", { class: "empty-state-title" }, "Пока ничего не выполнялось"),
            h("div", {}, "Перейдите на вкладку «Команды» и нажмите «Выполнить»"),
          ]),
        ]),
      ]);
      return;
    }

    mount(root, [
      h("div", { class: "page-header" }, [h("div", {}, [h("h1", { class: "page-title" }, "Вывод")])]),
      renderRunDetail(state.currentRun, openOutputModal),
    ]);
  }

  function statusLabel(status) {
    switch (status) {
      case "success":
        return "Успешно";
      case "error":
        return "Ошибка";
      case "running":
        return "Выполняется";
      default:
        return "В очереди";
    }
  }

  function renderRunDetail(meta, onDeviceClick) {
    const counts = { success: 0, error: 0, running: 0, pending: 0 };
    meta.devices.forEach((d) => {
      counts[d.status] = (counts[d.status] || 0) + 1;
    });

    const startedLabel = formatDateTime(meta.startedAt);
    const finishedLabel = meta.finishedAt ? formatDateTime(meta.finishedAt) : null;

    const deviceList = h(
      "div",
      { class: "device-list" },
      meta.devices.map((d) =>
        h(
          "button",
          {
            class: "device-status-row",
            onClick: () => onDeviceClick(meta.id, d.id, d.name),
          },
          [
            h("span", { class: "status-dot " + d.status }),
            h("div", { class: "device-status-info" }, [
              h(
                "div",
                { class: "device-status-name" },
                d.name + (d.address && d.address !== d.name ? " — " + d.address : "")
              ),
              d.error ? h("div", { class: "device-status-sub" }, d.error) : null,
            ]),
            h("span", { class: "status-label " + d.status }, statusLabel(d.status)),
            h("span", { class: "chevron" }, "›"),
          ]
        )
      )
    );

    return h("div", { class: "card" }, [
      h("div", { class: "run-summary" }, [
        h("div", {}, [
          h("p", { class: "run-summary-title" }, meta.description || "Без описания"),
          h(
            "p",
            { class: "run-summary-meta" },
            "Начало: " +
              startedLabel +
              (finishedLabel ? " · Завершено: " + finishedLabel : " · выполняется…") +
              (meta.cancelled ? " · отменено" : "")
          ),
        ]),
        h("div", { class: "counts" }, [
          counts.running ? h("span", { class: "count-pill running" }, counts.running + " выполняется") : null,
          counts.pending ? h("span", { class: "count-pill pending" }, counts.pending + " в очереди") : null,
          counts.success ? h("span", { class: "count-pill success" }, counts.success + " успешно") : null,
          counts.error ? h("span", { class: "count-pill error" }, counts.error + " ошибок") : null,
        ]),
      ]),
      deviceList,
    ]);
  }

  async function openOutputModal(runId, deviceId, name) {
    const body = h("div", {}, [h("div", { class: "hint" }, "Загрузка…")]);
    openModal({ title: name, body, wide: true });

    try {
      const text = await api.getDeviceOutput(runId, deviceId);
      clear(body);
      body.appendChild(h("pre", { class: "output-view" }, text || "(пусто)"));
    } catch (e) {
      clear(body);
      body.appendChild(h("div", { class: "output-error-banner" }, errMsg(e)));
    }
  }

  /* ============================================================
   * History tab
   * ============================================================ */

  function renderHistory() {
    const root = document.getElementById("tab-history");

    if (state.historyDetail) {
      mount(root, [
        h(
          "button",
          {
            class: "back-link",
            onClick: () => {
              state.historyDetail = null;
              renderHistory();
            },
          },
          "← Ко всем запускам"
        ),
        renderRunDetail(state.historyDetail, openOutputModal),
      ]);
      return;
    }

    if (!state.historyList.length) {
      mount(root, [
        h("div", { class: "page-header" }, [h("div", {}, [h("h1", { class: "page-title" }, "История")])]),
        h("div", { class: "card" }, [
          h("div", { class: "empty-state" }, [
            h("div", { class: "empty-state-title" }, "История пуста"),
            h("div", {}, "Здесь будут появляться результаты всех выполненных запусков"),
          ]),
        ]),
      ]);
      return;
    }

    const list = h(
      "div",
      { class: "history-list" },
      state.historyList.map((meta) => {
        const counts = { success: 0, error: 0 };
        meta.devices.forEach((d) => {
          if (d.status === "success") counts.success++;
          else if (d.status === "error") counts.error++;
        });

        return h(
          "button",
          { class: "history-row", onClick: () => openHistoryDetail(meta.id) },
          [
            h("div", {}, [
              h("div", { class: "history-row-title" }, meta.description || "Без описания"),
              h(
                "div",
                { class: "history-row-meta" },
                formatDateTime(meta.startedAt) + " · " + meta.devices.length + " устройств"
              ),
            ]),
            h("div", { class: "counts" }, [
              counts.success ? h("span", { class: "count-pill success" }, String(counts.success)) : null,
              counts.error ? h("span", { class: "count-pill error" }, String(counts.error)) : null,
            ]),
          ]
        );
      })
    );

    mount(root, [
      h("div", { class: "page-header" }, [h("div", {}, [h("h1", { class: "page-title" }, "История")])]),
      list,
    ]);
  }

  async function openHistoryDetail(runId) {
    try {
      const meta = await api.getRun(runId);
      state.historyDetail = meta;
      renderHistory();
    } catch (e) {
      toast(errMsg(e), "error");
    }
  }

  /* ============================================================
   * Settings tab
   * ============================================================ */

  function renderSettings() {
    const root = document.getElementById("tab-settings");

    if (!state.settings) {
      mount(root, [h("div", { class: "card" }, [h("div", { class: "hint" }, "Загрузка…")])]);
      loadSettings().then(() => {
        if (state.tab === "settings") renderSettings();
      });
      return;
    }

    const s = Object.assign({}, state.settings);

    const userInput = h("input", {
      type: "text",
      value: s.user,
      onInput: (e) => {
        s.user = e.target.value;
      },
    });

    const portInput = h("input", {
      type: "number",
      value: s.port,
      min: 1,
      max: 65535,
      onInput: (e) => {
        s.port = parseIntSafe(e.target.value, 22);
      },
    });

    const privateKeyInput = h("input", {
      type: "text",
      placeholder: "~/.ssh/id_rsa",
      value: s.privateKey,
      onInput: (e) => {
        s.privateKey = e.target.value;
      },
    });

    const passphraseInput = h("input", {
      type: "password",
      value: s.passphrase,
      onInput: (e) => {
        s.passphrase = e.target.value;
      },
    });

    const passwordInput = h("input", {
      type: "password",
      value: s.password,
      onInput: (e) => {
        s.password = e.target.value;
      },
    });

    const keyFields = h("div", {}, [
      h("div", { class: "field" }, [
        h("label", {}, "Путь к приватному ключу"),
        h("div", { class: "toolbar" }, [
          privateKeyInput,
          h(
            "button",
            {
              class: "btn btn-secondary",
              onClick: async () => {
                try {
                  const path = await api.pickPrivateKeyFile();
                  if (path) {
                    privateKeyInput.value = path;
                    s.privateKey = path;
                  }
                } catch (e) {
                  toast(errMsg(e), "error");
                }
              },
            },
            "Обзор…"
          ),
        ]),
      ]),
      h("div", { class: "field" }, [h("label", {}, "Пароль-фраза ключа (необязательно)"), passphraseInput]),
    ]);

    const passwordFields = h("div", { class: "field" }, [h("label", {}, "Пароль"), passwordInput]);

    const authFieldsWrap = h("div", {}, [s.authType === "password" ? passwordFields : keyFields]);

    const authSelect = h(
      "select",
      {
        onChange: (e) => {
          s.authType = e.target.value;
          clear(authFieldsWrap);
          authFieldsWrap.appendChild(s.authType === "password" ? passwordFields : keyFields);
        },
      },
      [
        h("option", { value: "key", selected: s.authType !== "password" }, "Ключ"),
        h("option", { value: "password", selected: s.authType === "password" }, "Пароль"),
      ]
    );

    const knownHostsInput = h("input", {
      type: "text",
      placeholder: "~/.ssh/known_hosts",
      value: s.knownHosts,
      onInput: (e) => {
        s.knownHosts = e.target.value;
      },
    });

    function buildKnownHostsField() {
      return h("div", { class: "field" }, [
        h("label", {}, "Файл known_hosts"),
        h("div", { class: "toolbar" }, [
          knownHostsInput,
          h(
            "button",
            {
              class: "btn btn-secondary",
              onClick: async () => {
                try {
                  const path = await api.pickKnownHostsFile();
                  if (path) {
                    knownHostsInput.value = path;
                    s.knownHosts = path;
                  }
                } catch (e) {
                  toast(errMsg(e), "error");
                }
              },
            },
            "Обзор…"
          ),
        ]),
      ]);
    }

    const strictWrap = h("div", {});
    if (s.strictHostKeyChecking) strictWrap.appendChild(buildKnownHostsField());

    const strictCheckbox = h("input", {
      type: "checkbox",
      checked: s.strictHostKeyChecking,
      onChange: (e) => {
        s.strictHostKeyChecking = e.target.checked;
        clear(strictWrap);
        if (s.strictHostKeyChecking) strictWrap.appendChild(buildKnownHostsField());
      },
    });

    const ptyCheckbox = h("input", {
      type: "checkbox",
      checked: s.pty,
      onChange: (e) => {
        s.pty = e.target.checked;
      },
    });

    const idleTimeoutInput = h("input", {
      type: "number",
      step: "0.5",
      min: "0.5",
      value: s.idleTimeoutSeconds,
      onInput: (e) => {
        s.idleTimeoutSeconds = parseFloatSafe(e.target.value, 2);
      },
    });

    const timeoutInput = h("input", {
      type: "number",
      min: "1",
      value: s.timeoutSeconds,
      onInput: (e) => {
        s.timeoutSeconds = parseIntSafe(e.target.value, 10);
      },
    });

    const workersInput = h("input", {
      type: "number",
      min: "1",
      value: s.workers,
      onInput: (e) => {
        s.workers = parseIntSafe(e.target.value, 10);
      },
    });

    const retryAttemptsInput = h("input", {
      type: "number",
      min: "0",
      value: s.retryAttempts,
      onInput: (e) => {
        s.retryAttempts = parseIntSafe(e.target.value, 0);
      },
    });

    const retryDelayInput = h("input", {
      type: "number",
      step: "0.5",
      min: "0",
      value: s.retryDelaySeconds,
      onInput: (e) => {
        s.retryDelaySeconds = parseFloatSafe(e.target.value, 0);
      },
    });

    const saveBtn = h(
      "button",
      {
        class: "btn btn-primary",
        onClick: async () => {
          try {
            const saved = await api.saveConnectionSettings(s);
            state.settings = saved;
            toast("Настройки сохранены", "success");
            renderSettings();
          } catch (e) {
            toast(errMsg(e), "error");
          }
        },
      },
      "Сохранить настройки"
    );

    mount(root, [
      h("div", { class: "page-header" }, [
        h("div", {}, [
          h("h1", { class: "page-title" }, "Настройки подключения"),
          h(
            "p",
            { class: "page-subtitle" },
            "Эти параметры применяются ко всем устройствам при запуске. Пароль и пароль-фраза хранятся локально в ~/SSHFleet/connection.json в открытом виде — по возможности используйте аутентификацию по ключу."
          ),
        ]),
      ]),
      h("div", { class: "card" }, [
        h("div", { class: "card-title" }, "Подключение"),
        h("div", { class: "field-row" }, [
          h("div", { class: "field" }, [h("label", {}, "Пользователь"), userInput]),
          h("div", { class: "field" }, [h("label", {}, "Порт по умолчанию"), portInput]),
        ]),
        h("div", { class: "field" }, [h("label", {}, "Тип аутентификации"), authSelect]),
        authFieldsWrap,
        h("div", { class: "field" }, [
          h("div", { class: "checkbox-row" }, [strictCheckbox, h("label", {}, "Строгая проверка ключа хоста")]),
        ]),
        strictWrap,
      ]),
      h("div", { class: "card" }, [
        h("div", { class: "card-title" }, "Режим выполнения"),
        h("div", { class: "field" }, [
          h("div", { class: "checkbox-row" }, [ptyCheckbox, h("label", {}, "Интерактивный режим (PTY)")]),
          h(
            "p",
            { class: "hint" },
            "Нужен для устройств, где по умолчанию доступен только режим exec (например Eltex) — позволяет входить в configure terminal и подобные режимы."
          ),
        ]),
        h("div", { class: "field-row" }, [
          h("div", { class: "field" }, [h("label", {}, "Тишина после команды, сек"), idleTimeoutInput]),
          h("div", { class: "field" }, [h("label", {}, "Таймаут подключения, сек"), timeoutInput]),
        ]),
      ]),
      h("div", { class: "card" }, [
        h("div", { class: "card-title" }, "Параллелизм и повторы"),
        h("div", { class: "field-row" }, [
          h("div", { class: "field" }, [h("label", {}, "Параллельных подключений"), workersInput]),
          h("div", { class: "field" }, [h("label", {}, "Повторов при ошибке подключения"), retryAttemptsInput]),
          h("div", { class: "field" }, [h("label", {}, "Задержка между повторами, сек"), retryDelayInput]),
        ]),
      ]),
      h("div", { class: "toolbar" }, [saveBtn]),
    ]);
  }

  /* ============================================================
   * Data loaders
   * ============================================================ */

  async function loadSettings() {
    try {
      state.settings = await api.getConnectionSettings();
    } catch (e) {
      toast(errMsg(e), "error");
    }
  }

  async function loadGroups() {
    try {
      state.groups = await api.getGroups();
    } catch (e) {
      toast(errMsg(e), "error");
    }
  }

  async function loadHistory() {
    try {
      state.historyList = await api.listHistory();
    } catch (e) {
      toast(errMsg(e), "error");
    }
  }

  /* ============================================================
   * Live progress events (Wails runtime)
   * ============================================================ */

  function updateDeviceInMeta(meta, device) {
    const idx = meta.devices.findIndex((d) => d.id === device.id);
    const devices = meta.devices.slice();

    if (idx === -1) devices.push(device);
    else devices[idx] = device;

    return Object.assign({}, meta, { devices });
  }

  function initRuntimeEvents() {
    if (!window.runtime || !window.runtime.EventsOn) return;

    window.runtime.EventsOn("run:progress", (evt) => {
      if (!evt || !state.currentRun || evt.runId !== state.currentRun.id) return;

      state.currentRun = updateDeviceInMeta(state.currentRun, evt.device);
      updateNavBadge();

      if (state.tab === "output") renderOutput();
    });

    window.runtime.EventsOn("run:finished", (meta) => {
      if (!meta || !state.currentRun || meta.id !== state.currentRun.id) return;

      state.currentRun = meta;
      updateNavBadge();

      if (state.tab === "output") renderOutput();
      if (state.tab === "commands") renderCommands();

      toast(meta.cancelled ? "Выполнение остановлено" : "Выполнение завершено", meta.cancelled ? undefined : "success");

      loadHistory();
    });
  }

  /* ============================================================
   * Boot
   * ============================================================ */

  function initNav() {
    document.querySelectorAll(".nav-item").forEach((btn) => {
      btn.addEventListener("click", () => setTab(btn.dataset.tab));
    });
  }

  async function boot() {
    initNav();
    initRuntimeEvents();
    setTab("devices");
    updateNavBadge();

    await Promise.all([loadGroups(), loadHistory(), loadSettings()]);

    renderActiveTab();
  }

  document.addEventListener("DOMContentLoaded", boot);
})();
