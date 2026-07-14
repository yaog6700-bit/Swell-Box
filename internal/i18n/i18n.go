package i18n

import "sync"

type Lang string

const (
	ZH Lang = "zh"
	EN Lang = "en"
)

var (
	mu   sync.RWMutex
	curr = ZH
)

func Set(lang Lang) {
	mu.Lock()
	defer mu.Unlock()
	if lang != ZH && lang != EN {
		lang = ZH
	}
	curr = lang
}

func Get() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return curr
}

func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()
	table := zh
	if curr == EN {
		table = en
	}
	if s, ok := table[key]; ok {
		return s
	}
	if s, ok := en[key]; ok {
		return s
	}
	return key
}

func IsZH() bool { return Get() == ZH }

var zh = map[string]string{
	"status_stopped":    "状态：已停止 ○",
	"status_running":    "状态：运行中 ●",
	"status_error":      "状态：错误 — ",
	"start":             "启动",
	"stop":              "停止",
	"restart":           "重启",
	"dashboard":         "面板",
	"menu_add":          "添加",
	"menu_settings":     "设置",
	"menu_tools":        "工具",
	"menu_nodes":        "节点",
	"nodes_empty":       "（暂无节点，请先导入）",
	"node_switched":     "已切换节点：",
	"node_switch_fail":  "切换节点失败：",
	"import_clipboard":  "导入节点（剪贴板）",
	"import_subscribe":  "导入订阅（剪贴板）",
	"update_subs":       "更新已存订阅",
	"sub_saved":         "已保存订阅：",
	"sub_updated":       "订阅已更新：%d 个节点",
	"sub_none":          "还没有保存的订阅",
	"config_reloaded":   "配置已变更，代理已重载",
	"config_reload_fail": "配置重载失败：",
	"import_config":     "导入配置文件…",
	"import_config_title": "选择 sing-box 配置 (JSON)",
	"autostart":         "开机自启",
	"auto_proxy":        "启动后自动连接",
	"system_proxy":      "系统代理",
	"tun_mode":          "TUN 模式（全局）",
	"tun_on":            "已开启 TUN：全局接管流量（已关闭系统代理）",
	"tun_off":           "已关闭 TUN 模式",
	"tun_admin_hint":    "提示：TUN 通常需要「以管理员身份运行」SWELL Box，否则可能启动失败",
	"tun_restarted":     "TUN 设置已变更，代理已重载",
	"configs":           "配置文件",
	"open_data":         "打开数据目录",
	"open_log":          "打开内核日志",
	"update":            "检查更新",
	"update_core":        "更新内核",
	"update_core_stable": "更新内核（稳定版）",
	"update_core_pre":    "更新内核（开发/预发布）",
	"update_geo":         "更新 Geo 规则",
	"update_app":         "检查程序更新",
	"language":          "语言 / Language",
	"lang_zh":           "中文",
	"lang_en":           "English",
	"about":             "关于 SWELL Box",
	"quit":              "退出",
	"tooltip_stopped":   "SWELL Box — 已停止",
	"tooltip_running":   "SWELL Box — 运行中",
	"app_running":       "SWELL Box 已启动，请查看系统托盘图标。",
	"starting":          "正在启动代理…",
	"started":           "代理已启动",
	"stopped":           "代理已停止",
	"restarting":        "正在重启代理…",
	"restarted":         "代理已重启",
	"start_failed":      "启动失败：",
	"stop_failed":       "停止失败：",
	"restart_failed":    "重启失败：",
	"import_empty":      "无法读取剪贴板",
	"import_failed":     "导入失败：",
	"save_failed":       "保存失败：",
	"imported":          "已导入：",
	"imported_n":        "已导入 %d 个节点",
	"imported_restart":  " — 已重启生效",
	"imported_start":    " — 请点「启动」使用",
	"sub_importing":     "正在下载订阅…",
	"sub_ok":            "订阅导入成功：%d 个节点",
	"sub_failed":        "订阅导入失败：",
	"cfg_import_ok":     "已导入配置：%s",
	"cfg_import_fail":   "导入配置失败：",
	"cfg_import_cancel": "已取消选择文件",
	"autostart_on":      "已开启开机自启",
	"autostart_off":     "已关闭开机自启",
	"autostart_fail":    "设置开机自启失败：",
	"auto_proxy_on":     "已开启：程序启动后自动连接代理",
	"auto_proxy_off":    "已关闭：启动后自动连接",
	"sysproxy_on":       "已开启系统代理",
	"sysproxy_off":      "已关闭系统代理",
	"sysproxy_fail":     "系统代理设置失败：",
	"sysproxy_off_for_tun": "TUN 模式下已自动关闭系统代理",
	"core_missing":      "未找到内核，正在自动下载…",
	"core_ready":        "内核已就绪：%s",
	"core_download_fail": "内核下载失败：",
	"upd_checking":      "正在检查更新…",
	"upd_core_ok":       "内核已更新到 %s，请重新启动代理",
	"upd_core_fail":     "内核更新失败：",
	"upd_core_latest":   "内核已是最新：%s",
	"upd_core_avail":    "发现新内核 %s（当前 %s），正在下载…",
	"upd_app_ver":       "程序版本：%s",
	"upd_app_manual":    "程序更新：当前 %s（发布渠道未配置，请手动替换 swellbox.exe）",
	"upd_app_latest":    "程序已是最新：%s",
	"upd_app_avail":      "发现新程序 %s，请打开下载页",
	"upd_geo_start":     "正在下载 Geo 规则…",
	"upd_geo_ok":        "Geo 规则已更新，请重启代理",
	"upd_geo_fail":      "Geo 规则更新失败：",
	"open_browser_fail": "无法自动打开浏览器。\n请手动打开：\n",
	"lang_switched":     "已切换为中文",
	"lang_switched_en":  "Switched to English",
	"no_config":         "（没有 config*.json）",
}

var en = map[string]string{
	"status_stopped":    "Status: Stopped ○",
	"status_running":    "Status: Running ●",
	"status_error":      "Status: Error — ",
	"start":             "Start",
	"stop":              "Stop",
	"restart":           "Restart",
	"dashboard":         "Dashboard",
	"menu_add":          "Add",
	"menu_settings":     "Settings",
	"menu_tools":        "Tools",
	"menu_nodes":        "Nodes",
	"nodes_empty":       "(no nodes — import first)",
	"node_switched":     "Switched to: ",
	"node_switch_fail":  "Switch node failed: ",
	"import_clipboard":  "Import Node (Clipboard)",
	"import_subscribe":  "Import Subscription (Clipboard)",
	"update_subs":       "Update Saved Subscriptions",
	"sub_saved":         "Subscription saved: ",
	"sub_updated":       "Subscription updated: %d nodes",
	"sub_none":          "No saved subscriptions",
	"config_reloaded":   "Config changed — proxy reloaded",
	"config_reload_fail": "Config reload failed: ",
	"import_config":     "Import Config File…",
	"import_config_title": "Select sing-box config (JSON)",
	"autostart":         "Launch at Login",
	"auto_proxy":        "Connect Proxy on Launch",
	"system_proxy":      "System Proxy",
	"tun_mode":          "TUN Mode (Global)",
	"tun_on":            "TUN on: global capture (system proxy disabled)",
	"tun_off":           "TUN mode off",
	"tun_admin_hint":    "Hint: TUN usually needs SWELL Box run as Administrator, or start may fail",
	"tun_restarted":     "TUN setting changed — proxy reloaded",
	"configs":           "Configs",
	"open_data":         "Open Data Folder",
	"open_log":          "Open Core Log",
	"update":            "Check for Updates",
	"update_core":        "Update Core",
	"update_core_stable": "Update Core (Stable)",
	"update_core_pre":    "Update Core (Pre-release)",
	"update_geo":         "Update Geo Rules",
	"update_app":         "Check App Update",
	"language":          "Language / 语言",
	"lang_zh":           "中文",
	"lang_en":           "English",
	"about":             "About SWELL Box",
	"quit":              "Quit",
	"tooltip_stopped":   "SWELL Box — stopped",
	"tooltip_running":   "SWELL Box — running",
	"app_running":       "SWELL Box is running. Check the system tray icon.",
	"starting":          "Starting proxy…",
	"started":           "Proxy started",
	"stopped":           "Proxy stopped",
	"restarting":        "Restarting proxy…",
	"restarted":         "Proxy restarted",
	"start_failed":      "Start failed: ",
	"stop_failed":       "Stop failed: ",
	"restart_failed":    "Restart failed: ",
	"import_empty":      "Cannot read clipboard",
	"import_failed":     "Import failed: ",
	"save_failed":       "Save failed: ",
	"imported":          "Imported: ",
	"imported_n":        "Imported %d nodes",
	"imported_restart":  " — restarted",
	"imported_start":    " — Start to use",
	"sub_importing":     "Downloading subscription…",
	"sub_ok":            "Subscription imported: %d nodes",
	"sub_failed":        "Subscription failed: ",
	"cfg_import_ok":     "Config imported: %s",
	"cfg_import_fail":   "Import config failed: ",
	"cfg_import_cancel": "File selection cancelled",
	"autostart_on":      "Launch at login enabled",
	"autostart_off":     "Launch at login disabled",
	"autostart_fail":    "Autostart failed: ",
	"auto_proxy_on":     "Will connect proxy when app launches",
	"auto_proxy_off":    "Will not auto-connect proxy",
	"sysproxy_on":       "System proxy enabled",
	"sysproxy_off":      "System proxy disabled",
	"sysproxy_fail":     "System proxy failed: ",
	"sysproxy_off_for_tun": "System proxy auto-disabled for TUN mode",
	"core_missing":      "Core not found, downloading…",
	"core_ready":        "Core ready: %s",
	"core_download_fail": "Core download failed: ",
	"upd_checking":      "Checking for updates…",
	"upd_core_ok":       "Core updated to %s — start proxy again",
	"upd_core_fail":     "Core update failed: ",
	"upd_core_latest":   "Core is up to date: %s",
	"upd_core_avail":    "New core %s (current %s), downloading…",
	"upd_app_ver":       "App version: %s",
	"upd_app_manual":    "App update: v%s (no release channel; replace swellbox.exe manually)",
	"upd_app_latest":    "App is up to date: %s",
	"upd_app_avail":      "New app %s available — opening download",
	"upd_geo_start":     "Downloading Geo rules…",
	"upd_geo_ok":        "Geo rules updated — restart proxy",
	"upd_geo_fail":      "Geo update failed: ",
	"open_browser_fail": "Cannot open browser automatically.\nPlease open manually:\n",
	"lang_switched":     "已切换为中文",
	"lang_switched_en":  "Switched to English",
	"no_config":         "(no config*.json)",
}
