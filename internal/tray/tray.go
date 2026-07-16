package tray

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"

	"github.com/swell-app/swellbox/internal/app"
	"github.com/swell-app/swellbox/internal/autostart"
	"github.com/swell-app/swellbox/internal/config"
	"github.com/swell-app/swellbox/internal/core"
	"github.com/swell-app/swellbox/internal/i18n"
	"github.com/swell-app/swellbox/internal/notify"
	"github.com/swell-app/swellbox/internal/paths"
	"github.com/swell-app/swellbox/internal/sharelink"
	"github.com/swell-app/swellbox/internal/subscribe"
	"github.com/swell-app/swellbox/internal/sysproxy"
	"github.com/swell-app/swellbox/internal/update"
	"github.com/swell-app/swellbox/internal/watch"
)

// Icons holds tray icon bytes.
type Icons struct {
	On  []byte // monochrome — system proxy / normal mode, running
	Off []byte // monochrome — stopped
	// Tun is the color brand logo used when TUN mode is on and proxy is running.
	// If empty, On is used as fallback.
	Tun []byte
}

// Controller wires tray UI to core manager.
type Controller struct {
	Icons Icons
	Core  *core.Manager
	App   *config.AppSettings

	mu           sync.Mutex
	mProxy       *systray.MenuItem
	mRestart     *systray.MenuItem
	mDashboard   *systray.MenuItem
	mStatus      *systray.MenuItem
	// Group parents (top-level)
	mSubs     *systray.MenuItem // 订阅：导入 / 更新 / 删除
	mSettings *systray.MenuItem
	mTools    *systray.MenuItem
	mNodes    *systray.MenuItem

	mImport        *systray.MenuItem // under 节点
	mSubscribe     *systray.MenuItem // under 订阅
	mUpdateSubs    *systray.MenuItem
	mImportConfig  *systray.MenuItem // under 配置文件
	mAutostart     *systray.MenuItem
	mAutoProxy     *systray.MenuItem
	mSysProxy      *systray.MenuItem
	mTunMode       *systray.MenuItem
	mConfigs       *systray.MenuItem
	mOpenDir       *systray.MenuItem
	mOpenLog       *systray.MenuItem
	mUpdate        *systray.MenuItem
	mUpdateCore    *systray.MenuItem
	mUpdateCorePre *systray.MenuItem
	mUpdateGeo     *systray.MenuItem
	mUpdateApp     *systray.MenuItem
	mLang          *systray.MenuItem
	mLangZH        *systray.MenuItem
	mLangEN        *systray.MenuItem
	mAbout         *systray.MenuItem
	mQuit          *systray.MenuItem

	// Dynamic config file slots under 配置文件
	configSlots     []*systray.MenuItem
	configNames     []string
	mConfigsEmpty   *systray.MenuItem
	mDeleteConfigs  *systray.MenuItem
	configDelSlots  []*systray.MenuItem
	configDelNames  []string
	mDeleteCfgEmpty *systray.MenuItem

	// Dynamic node slots (switch + delete)
	nodeSlots      []*systray.MenuItem
	nodeTags       []string
	nodeGroups     []string // clash group for each nodeTags[i] (region urltest or primary)
	mNodesEmpty    *systray.MenuItem
	mDeleteNodes   *systray.MenuItem
	deleteSlots    []*systray.MenuItem
	deleteTags     []string
	mDeleteEmpty   *systray.MenuItem
	selectorTag    string // top-level selector (proxy / Manual)

	// Saved subscription delete slots
	mDeleteSubs    *systray.MenuItem
	subDelSlots    []*systray.MenuItem
	subDelURLs     []string
	mDeleteSubEmpty *systray.MenuItem

	// Config file watcher
	cfgWatch       *watch.ConfigWatcher
	suppressReload time.Time
}

func (c *Controller) Run() {
	if c.App != nil {
		i18n.Set(i18n.Lang(c.App.Language))
	}
	systray.Run(c.onReady, c.onExit)
}

func (c *Controller) onReady() {
	// Keep tray icon-only. On macOS, SetTitle would show the app name next to the icon.
	systray.SetTitle("")
	systray.SetTooltip(i18n.TName("tooltip_stopped"))
	// Match original SingBoxClient: TemplateIcon + Icon for clear off/on colors.
	c.applyTrayIcon(false)

	// —— 主菜单保持精简 ——
	c.mStatus = systray.AddMenuItem(i18n.T("status_stopped"), "")
	c.mStatus.Disable()

	c.mProxy = systray.AddMenuItem(i18n.T("start"), "")
	c.mRestart = systray.AddMenuItem(i18n.T("restart"), "")
	c.mRestart.Hide()
	c.mDashboard = systray.AddMenuItem(i18n.T("dashboard"), "")

	systray.AddSeparator()

	// 节点 ▸ 切换 / 导入节点 / 删除节点
	c.initNodeSlots()

	// 订阅 ▸ 导入 / 更新 / 删除
	c.mSubs = systray.AddMenuItem(i18n.T("menu_subs"), "")
	c.mSubscribe = c.mSubs.AddSubMenuItem(i18n.T("import_subscribe"), "")
	c.mUpdateSubs = c.mSubs.AddSubMenuItem(i18n.T("update_subs"), "")
	c.initSubDeleteSlots()
	c.refreshSubDeleteMenu()

	// 配置文件 ▸ 列表 / 导入配置 / 删除配置
	c.mConfigs = systray.AddMenuItem(i18n.T("configs"), "")
	c.initConfigSlots()
	c.refreshConfigMenu()

	// 设置 ▸ 自启 / 代理 / 语言 / 工具（工具在设置内）
	c.mSettings = systray.AddMenuItem(i18n.T("menu_settings"), "")
	c.mAutostart = c.mSettings.AddSubMenuItemCheckbox(i18n.T("autostart"), "", autostart.Enabled())
	c.mAutoProxy = c.mSettings.AddSubMenuItemCheckbox(i18n.T("auto_proxy"), "", c.App != nil && c.App.AutoStartProxy)
	// TUN and system proxy are mutually exclusive in the UI.
	if c.App != nil && c.App.TunMode && c.App.SystemProxy {
		c.App.SystemProxy = false
		_ = config.SaveAppSettings(c.App)
	}
	sysProxyChecked := c.App != nil && c.App.SystemProxy && !c.App.TunMode
	c.mSysProxy = c.mSettings.AddSubMenuItemCheckbox(i18n.T("system_proxy"), "", sysProxyChecked)
	c.mTunMode = c.mSettings.AddSubMenuItemCheckbox(i18n.T("tun_mode"), "", c.App != nil && c.App.TunMode)
	c.mLang = c.mSettings.AddSubMenuItem(i18n.T("language"), "")
	c.mLangZH = c.mLang.AddSubMenuItemCheckbox(i18n.T("lang_zh"), "", i18n.Get() == i18n.ZH)
	c.mLangEN = c.mLang.AddSubMenuItemCheckbox(i18n.T("lang_en"), "", i18n.Get() == i18n.EN)

	// 工具（放在设置下面）▸ 数据目录 / 日志 / 检查更新
	c.mTools = c.mSettings.AddSubMenuItem(i18n.T("menu_tools"), "")
	c.mOpenDir = c.mTools.AddSubMenuItem(i18n.T("open_data"), "")
	c.mOpenLog = c.mTools.AddSubMenuItem(i18n.T("open_log"), "")
	c.mUpdate = c.mTools.AddSubMenuItem(i18n.T("update"), "")
	c.mUpdateCore = c.mUpdate.AddSubMenuItem(i18n.T("update_core_stable"), "")
	c.mUpdateCorePre = c.mUpdate.AddSubMenuItem(i18n.T("update_core_pre"), "")
	c.mUpdateGeo = c.mUpdate.AddSubMenuItem(i18n.T("update_geo"), "")
	c.mUpdateApp = c.mUpdate.AddSubMenuItem(i18n.T("update_app"), "")

	// 关于（放在设置下面）
	c.mAbout = c.mSettings.AddSubMenuItem(i18n.TName("about"), "")

	systray.AddSeparator()
	c.mQuit = systray.AddMenuItem(i18n.T("quit"), "")

	go c.loop()

	notify.Info(paths.AppName, i18n.TName("app_running"))

	// One-app experience: auto-fetch core on first run if missing.
	go c.ensureCoreAsync()

	// Watch active config for save → auto reload
	c.startConfigWatch()
	c.refreshNodeMenu()

	if c.App != nil && c.App.AutoStartProxy {
		go func() {
			time.Sleep(800 * time.Millisecond)
			if err := c.startProxy(); err != nil {
				log.Println("auto start:", err)
				c.setStatus(false, err.Error())
				notify.Error(paths.AppName, i18n.T("start_failed")+err.Error())
			} else {
				notify.Info(paths.AppName, i18n.T("started"))
			}
		}()
	}
}

func (c *Controller) loop() {
	for {
		select {
		case <-c.mProxy.ClickedCh:
			if c.Core.Running() {
				if err := c.stopProxy(); err != nil {
					notify.Error(paths.AppName, i18n.T("stop_failed")+err.Error())
				} else {
					notify.Info(paths.AppName, i18n.T("stopped"))
				}
			} else {
				notify.Info(paths.AppName, i18n.T("starting"))
				if err := c.startProxy(); err != nil {
					c.setStatus(false, err.Error())
					notify.Error(paths.AppName, i18n.T("start_failed")+err.Error())
				} else {
					notify.Info(paths.AppName, i18n.T("started"))
				}
			}
		case <-c.mRestart.ClickedCh:
			notify.Info(paths.AppName, i18n.T("restarting"))
			_ = c.stopProxy()
			if err := c.startProxy(); err != nil {
				c.setStatus(false, err.Error())
				notify.Error(paths.AppName, i18n.T("restart_failed")+err.Error())
			} else {
				notify.Info(paths.AppName, i18n.T("restarted"))
			}
		case <-c.mDashboard.ClickedCh:
			c.openDashboard()
		case <-c.mImport.ClickedCh:
			c.importFromClipboard()
		case <-c.mSubscribe.ClickedCh:
			c.importSubscription()
		case <-c.mUpdateSubs.ClickedCh:
			go c.updateSavedSubscriptions()
		case <-c.mSubs.ClickedCh:
			// Refresh delete list when user opens 订阅 menu.
			c.refreshSubDeleteMenu()
		case <-c.mImportConfig.ClickedCh:
			c.importConfigFile()
		case <-c.mAutostart.ClickedCh:
			c.toggleAutostart()
		case <-c.mAutoProxy.ClickedCh:
			c.toggleAutoProxy()
		case <-c.mSysProxy.ClickedCh:
			c.toggleSysProxy()
		case <-c.mTunMode.ClickedCh:
			c.toggleTunMode()
		case <-c.mOpenDir.ClickedCh:
			if dir, err := paths.HomeDir(); err == nil {
				_ = app.OpenPath(dir)
			}
		case <-c.mOpenLog.ClickedCh:
			if dir, err := paths.LogsDir(); err == nil {
				_ = app.OpenPath(filepath.Join(dir, "core.log"))
			}
		case <-c.mUpdateCore.ClickedCh:
			go c.doUpdateCore(update.ChannelStable)
		case <-c.mUpdateCorePre.ClickedCh:
			go c.doUpdateCore(update.ChannelPre)
		case <-c.mUpdateGeo.ClickedCh:
			go c.doUpdateGeo()
		case <-c.mUpdateApp.ClickedCh:
			go c.doCheckApp()
		case <-c.mLangZH.ClickedCh:
			c.switchLang(i18n.ZH)
		case <-c.mLangEN.ClickedCh:
			c.switchLang(i18n.EN)
		case <-c.mAbout.ClickedCh:
			// Project homepage (not the sing-box core repo)
			aboutURL := "https://github.com/yaog6700-bit/Swell-Box"
			if update.AppReleaseRepo != "" {
				aboutURL = "https://github.com/" + update.AppReleaseRepo
			}
			_ = app.OpenURL(aboutURL)
		case <-c.mQuit.ClickedCh:
			go func() {
				c.shutdown()
				systray.Quit()
				time.AfterFunc(500*time.Millisecond, func() { os.Exit(0) })
			}()
			return
		case <-c.mConfigs.ClickedCh:
			c.refreshConfigMenu()
		case <-c.mUpdate.ClickedCh:
		case <-c.mLang.ClickedCh:
		case <-c.mSettings.ClickedCh:
		case <-c.mTools.ClickedCh:
		case <-c.mNodes.ClickedCh:
			// refresh when user opens nodes menu (best-effort)
			c.refreshNodeMenu()
		}
	}
}

func (c *Controller) startConfigWatch() {
	w, err := watch.New(func() {
		c.onConfigFileChanged()
	})
	if err != nil {
		log.Println("watch:", err)
		return
	}
	c.cfgWatch = w
	c.cfgWatch.Start()
	c.rewatchActiveConfig()
}

func (c *Controller) rewatchActiveConfig() {
	if c.cfgWatch == nil || c.App == nil {
		return
	}
	path, err := config.ActiveConfigPath(c.App)
	if err != nil {
		return
	}
	_ = c.cfgWatch.SetPath(path)
}

func (c *Controller) suppressConfigWatch(d time.Duration) {
	c.suppressReload = time.Now().Add(d)
}

func (c *Controller) onConfigFileChanged() {
	if time.Now().Before(c.suppressReload) {
		return
	}
	if c.Core == nil || !c.Core.Running() {
		c.refreshNodeMenu()
		return
	}
	// Reload proxy with new config
	c.suppressConfigWatch(2 * time.Second)
	_ = c.stopProxy()
	if err := c.startProxy(); err != nil {
		notify.Error(paths.AppName, i18n.T("config_reload_fail")+err.Error())
		return
	}
	notify.Info(paths.AppName, i18n.T("config_reloaded"))
	c.refreshNodeMenu()
}

func (c *Controller) toggleAutostart() {
	on := !autostart.Enabled()
	if err := autostart.Set(on); err != nil {
		notify.Error(paths.AppName, i18n.T("autostart_fail")+err.Error())
		if autostart.Enabled() {
			c.mAutostart.Check()
		} else {
			c.mAutostart.Uncheck()
		}
		return
	}
	if on {
		c.mAutostart.Check()
		notify.Info(paths.AppName, i18n.T("autostart_on"))
	} else {
		c.mAutostart.Uncheck()
		notify.Info(paths.AppName, i18n.T("autostart_off"))
	}
}

func (c *Controller) toggleAutoProxy() {
	if c.App == nil {
		return
	}
	c.App.AutoStartProxy = !c.App.AutoStartProxy
	_ = config.SaveAppSettings(c.App)
	if c.App.AutoStartProxy {
		c.mAutoProxy.Check()
		notify.Info(paths.AppName, i18n.T("auto_proxy_on"))
	} else {
		c.mAutoProxy.Uncheck()
		notify.Info(paths.AppName, i18n.T("auto_proxy_off"))
	}
}

func (c *Controller) toggleSysProxy() {
	if c.App == nil {
		return
	}
	// Enabling system proxy turns off TUN (they conflict for most users).
	want := !c.App.SystemProxy
	if want && c.App.TunMode {
		c.App.TunMode = false
		if c.mTunMode != nil {
			c.mTunMode.Uncheck()
		}
		c.refreshTrayIcon()
		if c.Core != nil && c.Core.Running() {
			c.reloadProxyForTun()
		}
	}
	c.App.SystemProxy = want
	_ = config.SaveAppSettings(c.App)
	if c.App.SystemProxy {
		c.mSysProxy.Check()
		if c.Core.Running() {
			if err := sysproxy.Enable(c.proxyAddr()); err != nil {
				notify.Error(paths.AppName, i18n.T("sysproxy_fail")+err.Error())
				return
			}
		}
		notify.Info(paths.AppName, i18n.T("sysproxy_on"))
	} else {
		c.mSysProxy.Uncheck()
		_ = sysproxy.Restore()
		notify.Info(paths.AppName, i18n.T("sysproxy_off"))
	}
}

func (c *Controller) toggleTunMode() {
	if c.App == nil {
		return
	}
	want := !c.App.TunMode

	// Enabling TUN without admin: prompt for UAC relaunch instead of failing later.
	if want && !app.IsElevated() {
		if !app.ConfirmYesNo(i18n.T("tun_elevate_title"), i18n.T("tun_elevate_body")) {
			// Keep menu unchecked; nothing saved.
			if c.mTunMode != nil {
				c.mTunMode.Uncheck()
			}
			notify.Info(paths.AppName, i18n.T("tun_elevate_cancel"))
			return
		}
		// Persist TUN on, drop system proxy, then elevate-restart.
		c.App.TunMode = true
		if c.App.SystemProxy {
			c.App.SystemProxy = false
			if c.mSysProxy != nil {
				c.mSysProxy.Uncheck()
			}
			_ = sysproxy.Restore()
		}
		if c.mTunMode != nil {
			c.mTunMode.Check()
		}
		_ = config.SaveAppSettings(c.App)
		c.elevateAndExit()
		return
	}

	c.App.TunMode = want
	if c.App.TunMode {
		// TUN captures traffic at OS level — turn off system HTTP proxy.
		if c.App.SystemProxy {
			c.App.SystemProxy = false
			if c.mSysProxy != nil {
				c.mSysProxy.Uncheck()
			}
			_ = sysproxy.Restore()
		}
		if c.mTunMode != nil {
			c.mTunMode.Check()
		}
		_ = config.SaveAppSettings(c.App)
		notify.Info(paths.AppName, i18n.T("tun_on"))
	} else {
		if c.mTunMode != nil {
			c.mTunMode.Uncheck()
		}
		_ = config.SaveAppSettings(c.App)
		notify.Info(paths.AppName, i18n.T("tun_off"))
	}
	// Icon reflects TUN only while running; refresh immediately (reload also re-applies).
	c.refreshTrayIcon()
	if c.Core != nil && c.Core.Running() {
		c.reloadProxyForTun()
	}
}

// elevateAndExit stops proxy, requests UAC relaunch, then exits this process.
func (c *Controller) elevateAndExit() {
	_ = c.stopProxy()
	_ = sysproxy.Restore()
	if err := app.RelaunchElevated(); err != nil {
		// User cancelled UAC or ShellExecute failed — roll back TUN flag.
		if c.App != nil {
			c.App.TunMode = false
			_ = config.SaveAppSettings(c.App)
		}
		if c.mTunMode != nil {
			c.mTunMode.Uncheck()
		}
		c.refreshTrayIcon()
		if err.Error() == "uac cancelled" {
			notify.Info(paths.AppName, i18n.T("tun_elevate_cancel"))
			return
		}
		notify.Error(paths.AppName, i18n.T("tun_elevate_fail")+err.Error())
		return
	}
	// Elevated instance is starting; quit this non-admin process.
	systray.Quit()
}

func (c *Controller) reloadProxyForTun() {
	c.suppressConfigWatch(2 * time.Second)
	_ = c.stopProxy()
	if err := c.startProxy(); err != nil {
		c.setStatus(false, err.Error())
		notify.Error(paths.AppName, i18n.T("start_failed")+err.Error())
		return
	}
	notify.Info(paths.AppName, i18n.T("tun_restarted"))
}

func (c *Controller) proxyAddr() string {
	// Default mixed inbound in our seed config.
	return "127.0.0.1:7890"
}

func (c *Controller) coreChannel() string {
	if c.App != nil && c.App.CoreChannel == update.ChannelStable {
		return update.ChannelStable
	}
	return update.ChannelPre
}

func (c *Controller) ensureCoreAsync() {
	// Prefer copying bundled core next to exe (offline package).
	if ok, _ := update.InstallBundledCore(); ok {
		notify.Info(paths.AppName, fmt.Sprintf(i18n.T("core_ready"), "bundled"))
		return
	}
	if update.CorePresent() {
		return
	}
	notify.Info(paths.AppName, i18n.T("core_missing"))
	ver, err := update.EnsureCore(c.coreChannel(), nil)
	if err != nil {
		// Offline without bundle: warn only; Start will error with clear message.
		notify.Error(paths.AppName, i18n.T("core_download_fail")+err.Error())
		return
	}
	notify.Info(paths.AppName, fmt.Sprintf(i18n.T("core_ready"), ver))
}

func (c *Controller) ensureCoreSync() error {
	// 1) install from same folder as Swell-Box.exe (full zip package)
	if _, err := update.InstallBundledCore(); err == nil && update.CorePresent() {
		return nil
	}
	if update.CorePresent() {
		return nil
	}
	// 2) online download (needs network)
	notify.Info(paths.AppName, i18n.T("core_missing"))
	ver, err := update.EnsureCore(c.coreChannel(), nil)
	if err != nil {
		return err
	}
	notify.Info(paths.AppName, fmt.Sprintf(i18n.T("core_ready"), ver))
	return nil
}

func (c *Controller) applySystemProxy(on bool) {
	if c.App == nil || !c.App.SystemProxy || c.App.TunMode {
		if !on || (c.App != nil && c.App.TunMode) {
			_ = sysproxy.Restore()
		}
		return
	}
	if on {
		_ = sysproxy.Enable(c.proxyAddr())
	} else {
		_ = sysproxy.Restore()
	}
}

func (c *Controller) shutdown() {
	_ = c.stopProxy()
	_ = sysproxy.Restore()
}

// openDashboard opens the official sing-box dashboard in the browser.
// The API only listens while the core is running — start it first if needed.
func (c *Controller) openDashboard() {
	port := paths.DefaultPort
	if c.App != nil && c.App.DashboardPort > 0 {
		port = c.App.DashboardPort
	}
	url := paths.DashboardURL(port)

	if c.Core == nil || !c.Core.Running() {
		notify.Info(paths.AppName, i18n.T("dashboard_starting"))
		if err := c.startProxy(); err != nil {
			notify.Error(paths.AppName, i18n.T("dashboard_need_start")+err.Error())
			return
		}
		// Give API a moment to bind before the browser loads.
		time.Sleep(400 * time.Millisecond)
	}
	if err := app.OpenURL(url); err != nil {
		go popup(paths.AppName, i18n.T("open_browser_fail")+url)
	}
}

func (c *Controller) importConfigFile() {
	path, err := app.PickJSONFile(i18n.T("import_config_title"))
	if err != nil {
		notify.Error(paths.AppName, i18n.T("cfg_import_fail")+err.Error())
		return
	}
	if strings.TrimSpace(path) == "" {
		notify.Info(paths.AppName, i18n.T("cfg_import_cancel"))
		return
	}
	c.suppressConfigWatch(3 * time.Second)
	res, err := config.ImportConfigFile(c.App, path)
	if err != nil {
		notify.Error(paths.AppName, i18n.T("cfg_import_fail")+err.Error())
		return
	}
	c.rewatchActiveConfig()
	c.refreshConfigMenu() // show newly imported file without restarting the app
	c.refreshNodeMenu()
	msg := fmt.Sprintf(i18n.T("cfg_import_ok"), res.Name)
	if len(res.Warnings) > 0 {
		// e.g. empty urltest groups auto-filled with direct/first proxy
		msg += " — " + fmt.Sprintf(i18n.T("cfg_import_fixed"), len(res.Warnings))
	}
	if c.Core.Running() {
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			notify.Error(paths.AppName, msg+" — "+err.Error())
			return
		}
		notify.Info(paths.AppName, msg+i18n.T("imported_restart"))
		return
	}
	notify.Info(paths.AppName, msg+i18n.T("imported_start"))
}

func (c *Controller) importSubscription() {
	// 节点 / 订阅 always target the home profile config.json.
	if err := c.useHomeProfileForNodes(); err != nil {
		notify.Error(paths.AppName, i18n.T("save_failed")+err.Error())
		return
	}
	text, err := sharelink.ReadClipboard()
	if err != nil {
		notify.Error(paths.AppName, i18n.T("import_empty"))
		return
	}
	text = strings.TrimSpace(text)
	notify.Info(paths.AppName, i18n.T("sub_importing"))

	var (
		nodes   []sharelink.Node
		subURL  string // non-empty when fetched from a live subscription URL
	)
	nodes, err = subscribe.FetchURL(text)
	if err != nil {
		if nodes2, err2 := subscribe.ParseBody(text); err2 == nil {
			nodes = nodes2
		} else {
			notify.Error(paths.AppName, i18n.T("sub_failed")+err.Error())
			return
		}
	} else {
		subURL = text
		// Drop previous nodes from this URL before re-import (avoids -2 duplicates).
		if prev := config.GetSubscription(subURL); prev != nil && len(prev.NodeTags) > 0 {
			c.suppressConfigWatch(3 * time.Second)
			_, _ = config.RemoveNodesFromConfig(c.App, prev.NodeTags)
		}
		if s, err := config.AddSubscription(subURL); err == nil {
			notify.Info(paths.AppName, i18n.T("sub_saved")+s.Name)
			c.refreshSubDeleteMenu()
		}
	}
	tags := c.applyImportedNodes(nodes)
	if subURL != "" && len(tags) > 0 {
		_ = config.SetSubscriptionNodeTags(subURL, tags)
		c.refreshSubDeleteMenu()
	}
}

func (c *Controller) updateSavedSubscriptions() {
	if err := c.useHomeProfileForNodes(); err != nil {
		notify.Error(paths.AppName, i18n.T("save_failed")+err.Error())
		return
	}
	items, err := config.LoadSubscriptions()
	if err != nil || len(items) == 0 {
		notify.Info(paths.AppName, i18n.T("sub_none"))
		return
	}
	notify.Info(paths.AppName, i18n.T("sub_importing"))
	c.suppressConfigWatch(5 * time.Second)

	total := 0
	for _, it := range items {
		nodes, err := subscribe.FetchURL(it.URL)
		if err != nil {
			log.Println("sub update", it.URL, err)
			continue
		}
		// Replace previous nodes from this subscription.
		if len(it.NodeTags) > 0 {
			_, _ = config.RemoveNodesFromConfig(c.App, it.NodeTags)
		}
		tags, err := config.AddNodesToActiveConfig(c.App, nodes)
		if err != nil {
			log.Println("sub update apply", it.URL, err)
			continue
		}
		_ = config.SetSubscriptionNodeTags(it.URL, tags)
		total += len(tags)
	}
	if total == 0 {
		notify.Error(paths.AppName, i18n.T("sub_failed")+"no nodes")
		return
	}
	if c.Core != nil && c.Core.Running() {
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			notify.Error(paths.AppName, fmt.Sprintf(i18n.T("sub_updated"), total)+" — "+err.Error())
			c.refreshNodeMenu()
			return
		}
	}
	c.refreshNodeMenu()
	c.refreshSubDeleteMenu()
	notify.Info(paths.AppName, fmt.Sprintf(i18n.T("sub_updated"), total))
}

// useHomeProfileForNodes switches ActiveConfig to config.json so 节点/订阅
// never write into an imported full template.
func (c *Controller) useHomeProfileForNodes() error {
	if c.App == nil {
		return fmt.Errorf("no app settings")
	}
	if !config.IsDefaultConfigName(c.App.ActiveConfig) {
		c.App.ActiveConfig = config.DefaultConfigFile
		if err := config.SaveAppSettings(c.App); err != nil {
			return err
		}
		c.rewatchActiveConfig()
		c.refreshConfigMenu()
	}
	// Ensure home file exists (bootstrap usually created it).
	path, err := config.ActiveConfigPath(c.App)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("default %s missing — reopen app to recreate", config.DefaultConfigFile)
	}
	return nil
}

// applyImportedNodes writes nodes into the home profile and returns their tags.
func (c *Controller) applyImportedNodes(nodes []sharelink.Node) []string {
	if err := c.useHomeProfileForNodes(); err != nil {
		notify.Error(paths.AppName, i18n.T("save_failed")+err.Error())
		return nil
	}
	c.suppressConfigWatch(3 * time.Second)
	tags, err := config.AddNodesToActiveConfig(c.App, nodes)
	if err != nil {
		notify.Error(paths.AppName, i18n.T("save_failed")+err.Error())
		return nil
	}
	msg := fmt.Sprintf(i18n.T("sub_ok"), len(tags))
	if c.Core != nil && c.Core.Running() {
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			notify.Error(paths.AppName, msg)
			c.refreshNodeMenu()
			return tags
		}
		notify.Info(paths.AppName, msg+i18n.T("imported_restart"))
	} else {
		notify.Info(paths.AppName, msg+i18n.T("imported_start"))
	}
	c.refreshNodeMenu()
	return tags
}

func (c *Controller) doUpdateCore(channel string) {
	notify.Info(paths.AppName, i18n.T("upd_checking"))
	info, err := update.CheckCore(channel)
	if err != nil {
		notify.Error(paths.AppName, i18n.T("upd_core_fail")+err.Error())
		return
	}
	cur := info.Installed
	if cur == "" {
		cur = "?"
	}
	// Same version string → skip (allows switching stable↔pre when tags differ).
	if info.Latest != "" && cur == info.Latest {
		notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_core_latest"), cur))
		return
	}
	notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_core_avail"), info.Latest, cur))
	ver, err := update.UpdateCore(channel, func() error {
		return c.stopProxy()
	})
	if err != nil {
		notify.Error(paths.AppName, i18n.T("upd_core_fail")+err.Error())
		return
	}
	notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_core_ok"), ver))
}

func (c *Controller) doUpdateGeo() {
	notify.Info(paths.AppName, i18n.T("upd_geo_start"))
	if err := update.UpdateGeoRules(); err != nil {
		notify.Error(paths.AppName, i18n.T("upd_geo_fail")+err.Error())
		return
	}
	// Reload proxy so new rule-sets take effect if running.
	if c.Core != nil && c.Core.Running() {
		c.suppressConfigWatch(2 * time.Second)
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			notify.Info(paths.AppName, i18n.T("upd_geo_ok"))
			notify.Error(paths.AppName, i18n.T("start_failed")+err.Error())
			return
		}
	}
	notify.Info(paths.AppName, i18n.T("upd_geo_ok"))
}

func (c *Controller) doCheckApp() {
	notify.Info(paths.AppName, i18n.T("upd_checking"))
	res := update.CheckApp()
	if res.Message == "manual" || update.AppReleaseRepo == "" {
		notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_app_manual"), res.Current))
		return
	}
	if res.Message != "" && !res.HasUpdate {
		// network / API error
		notify.Error(paths.AppName, i18n.T("upd_app_fail")+res.Message)
		return
	}
	if !res.HasUpdate {
		if res.Latest != "" {
			notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_app_latest"), res.Current))
			return
		}
		notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_app_ver"), res.Current))
		return
	}

	// Has update
	if res.DownloadURL == "" {
		notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_app_open_page"), res.Latest))
		_ = app.OpenURL("https://github.com/" + update.AppReleaseRepo + "/releases")
		return
	}

	notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_app_downloading"), res.Latest))
	err := update.ApplyAppUpdate(res.DownloadURL, res.IsZip, func() error {
		return c.stopProxy()
	})
	if err != nil {
		// Fallback: open browser for manual download
		notify.Error(paths.AppName, i18n.T("upd_app_fail")+err.Error())
		notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_app_open_page"), res.Latest))
		_ = app.OpenURL(res.DownloadURL)
		return
	}
	notify.Info(paths.AppName, fmt.Sprintf(i18n.T("upd_app_ok"), res.Latest))
	// Exit so the updater can replace the locked .exe and relaunch.
	go func() {
		time.Sleep(800 * time.Millisecond)
		c.shutdown()
		systray.Quit()
		time.AfterFunc(400*time.Millisecond, func() { os.Exit(0) })
	}()
}

// local helper to avoid exporting versionLess from update with wrong name
func updateVersionLess(a, b string) bool {
	return update.VersionLess(a, b)
}

func (c *Controller) switchLang(lang i18n.Lang) {
	i18n.Set(lang)
	if c.App != nil {
		c.App.Language = string(lang)
		_ = config.SaveAppSettings(c.App)
	}
	c.applyMenuLanguage()
	if lang == i18n.ZH {
		c.mLangZH.Check()
		c.mLangEN.Uncheck()
		notify.Info(paths.AppName, i18n.T("lang_switched"))
	} else {
		c.mLangEN.Check()
		c.mLangZH.Uncheck()
		notify.Info(paths.AppName, i18n.T("lang_switched_en"))
	}
}

func (c *Controller) applyMenuLanguage() {
	running := c.Core != nil && c.Core.Running()
	set := func(m *systray.MenuItem, key string) {
		if m != nil {
			m.SetTitle(i18n.T(key))
		}
	}
	if c.mStatus != nil {
		if running {
			c.mStatus.SetTitle(i18n.T("status_running"))
		} else {
			c.mStatus.SetTitle(i18n.T("status_stopped"))
		}
	}
	if c.mProxy != nil {
		if running {
			c.mProxy.SetTitle(i18n.T("stop"))
		} else {
			c.mProxy.SetTitle(i18n.T("start"))
		}
	}
	set(c.mRestart, "restart")
	set(c.mDashboard, "dashboard")
	set(c.mNodes, "menu_nodes")
	set(c.mDeleteNodes, "menu_delete_node")
	set(c.mSubs, "menu_subs")
	set(c.mSubscribe, "import_subscribe")
	set(c.mUpdateSubs, "update_subs")
	set(c.mDeleteSubs, "menu_delete_sub")
	set(c.mImport, "import_clipboard")
	set(c.mImportConfig, "import_config")
	c.refreshSubDeleteMenu()
	set(c.mConfigs, "configs")
	set(c.mDeleteConfigs, "menu_delete_config")
	set(c.mSettings, "menu_settings")
	set(c.mAutostart, "autostart")
	set(c.mAutoProxy, "auto_proxy")
	set(c.mSysProxy, "system_proxy")
	set(c.mTunMode, "tun_mode")
	set(c.mLang, "language")
	set(c.mLangZH, "lang_zh")
	set(c.mLangEN, "lang_en")
	set(c.mUpdate, "update")
	set(c.mUpdateCore, "update_core_stable")
	set(c.mUpdateCorePre, "update_core_pre")
	set(c.mUpdateGeo, "update_geo")
	set(c.mUpdateApp, "update_app")
	set(c.mTools, "menu_tools")
	set(c.mOpenDir, "open_data")
	set(c.mOpenLog, "open_log")
	if c.mAbout != nil {
		c.mAbout.SetTitle(i18n.TName("about"))
	}
	set(c.mQuit, "quit")
	if running {
		systray.SetTooltip(i18n.TName("tooltip_running"))
	} else {
		systray.SetTooltip(i18n.TName("tooltip_stopped"))
	}
}

func (c *Controller) importFromClipboard() {
	if err := c.useHomeProfileForNodes(); err != nil {
		notify.Error(paths.AppName, i18n.T("save_failed")+err.Error())
		return
	}
	text, err := sharelink.ReadClipboard()
	if err != nil {
		notify.Error(paths.AppName, i18n.T("import_empty"))
		return
	}
	nodes, err := sharelink.Parse(text)
	if err != nil {
		notify.Error(paths.AppName, i18n.T("import_failed")+err.Error())
		return
	}
	c.suppressConfigWatch(3 * time.Second)
	tags, err := config.AddNodesToActiveConfig(c.App, nodes)
	if err != nil {
		notify.Error(paths.AppName, i18n.T("save_failed")+err.Error())
		return
	}
	msg := i18n.T("imported") + tags[0]
	if len(tags) > 1 {
		msg = fmt.Sprintf(i18n.T("imported_n"), len(tags))
	}
	if c.Core.Running() {
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			notify.Error(paths.AppName, msg)
			return
		}
		notify.Info(paths.AppName, msg+i18n.T("imported_restart"))
	} else {
		notify.Info(paths.AppName, msg+i18n.T("imported_start"))
	}
	c.refreshNodeMenu()
}

const maxConfigSlots = 24

func (c *Controller) initConfigSlots() {
	c.configSlots = make([]*systray.MenuItem, maxConfigSlots)
	c.configNames = make([]string, maxConfigSlots)
	for i := 0; i < maxConfigSlots; i++ {
		mi := c.mConfigs.AddSubMenuItemCheckbox("—", "", false)
		mi.Hide()
		c.configSlots[i] = mi
		idx := i
		go func() {
			for range mi.ClickedCh {
				c.onConfigClick(idx)
			}
		}()
	}
	c.mConfigsEmpty = c.mConfigs.AddSubMenuItem(i18n.T("no_config"), "")
	c.mConfigsEmpty.Disable()

	// 导入配置文件 — 与订阅分组方式一致，放在「配置文件」里
	c.mImportConfig = c.mConfigs.AddSubMenuItem(i18n.T("import_config"), "")

	// 删除配置文件 ▸
	c.mDeleteConfigs = c.mConfigs.AddSubMenuItem(i18n.T("menu_delete_config"), "")
	c.configDelSlots = make([]*systray.MenuItem, maxConfigSlots)
	c.configDelNames = make([]string, maxConfigSlots)
	for i := 0; i < maxConfigSlots; i++ {
		mi := c.mDeleteConfigs.AddSubMenuItem("—", "")
		mi.Hide()
		c.configDelSlots[i] = mi
		idx := i
		go func() {
			for range mi.ClickedCh {
				c.onConfigDelete(idx)
			}
		}()
	}
	c.mDeleteCfgEmpty = c.mDeleteConfigs.AddSubMenuItem(i18n.T("no_config"), "")
	c.mDeleteCfgEmpty.Disable()
}

func (c *Controller) onConfigClick(idx int) {
	if idx < 0 || idx >= len(c.configNames) {
		return
	}
	name := c.configNames[idx]
	if name == "" {
		return
	}
	c.selectConfig(name)
	// Update checkmarks immediately.
	active := ""
	if c.App != nil {
		active = c.App.ActiveConfig
	}
	for i, mi := range c.configSlots {
		if mi == nil || c.configNames[i] == "" {
			continue
		}
		if c.configNames[i] == active {
			mi.Check()
		} else {
			mi.Uncheck()
		}
	}
}

// refreshConfigMenu reloads the 配置文件 submenu from disk (import / switch / open).
func (c *Controller) refreshConfigMenu() {
	if c.mConfigs == nil || c.configSlots == nil {
		return
	}
	names, err := config.ListConfigFiles()
	if err != nil {
		names = nil
	}
	active := ""
	if c.App != nil {
		active = c.App.ActiveConfig
	}

	if len(names) == 0 {
		if c.mConfigsEmpty != nil {
			c.mConfigsEmpty.SetTitle(i18n.T("no_config"))
			c.mConfigsEmpty.Show()
		}
		for i, mi := range c.configSlots {
			c.configNames[i] = ""
			if mi != nil {
				mi.Hide()
			}
		}
		c.fillConfigDeleteSlots(nil)
		return
	}
	if c.mConfigsEmpty != nil {
		c.mConfigsEmpty.Hide()
	}

	for i := 0; i < maxConfigSlots; i++ {
		mi := c.configSlots[i]
		if mi == nil {
			continue
		}
		if i >= len(names) {
			c.configNames[i] = ""
			mi.Hide()
			continue
		}
		name := names[i]
		c.configNames[i] = name
		mi.SetTitle(name)
		mi.Show()
		if name == active {
			mi.Check()
		} else {
			mi.Uncheck()
		}
	}
	c.fillConfigDeleteSlots(names)
}

func (c *Controller) fillConfigDeleteSlots(names []string) {
	if c.mDeleteConfigs == nil {
		return
	}
	if len(names) == 0 {
		if c.mDeleteCfgEmpty != nil {
			c.mDeleteCfgEmpty.SetTitle(i18n.T("no_config"))
			c.mDeleteCfgEmpty.Show()
		}
		for i, mi := range c.configDelSlots {
			c.configDelNames[i] = ""
			if mi != nil {
				mi.Hide()
			}
		}
		return
	}
	if c.mDeleteCfgEmpty != nil {
		c.mDeleteCfgEmpty.Hide()
	}
	for i := 0; i < maxConfigSlots; i++ {
		mi := c.configDelSlots[i]
		if mi == nil {
			continue
		}
		if i >= len(names) {
			c.configDelNames[i] = ""
			mi.Hide()
			continue
		}
		c.configDelNames[i] = names[i]
		mi.SetTitle(names[i])
		mi.Show()
	}
}

func (c *Controller) onConfigDelete(idx int) {
	if idx < 0 || idx >= len(c.configDelNames) {
		return
	}
	name := c.configDelNames[idx]
	if name == "" {
		return
	}
	wasActive := c.App != nil && c.App.ActiveConfig == name
	running := c.Core != nil && c.Core.Running()

	c.suppressConfigWatch(3 * time.Second)
	if err := config.DeleteConfigFile(c.App, name); err != nil {
		notify.Error(paths.AppName, i18n.T("cfg_delete_fail")+err.Error())
		return
	}
	c.refreshConfigMenu()
	c.rewatchActiveConfig()
	c.refreshNodeMenu()

	if wasActive && running {
		_ = c.stopProxy()
		// If another config remains, try start with it.
		if names, err := config.ListConfigFiles(); err == nil && len(names) > 0 {
			if err := c.startProxy(); err != nil {
				notify.Error(paths.AppName, i18n.T("cfg_deleted")+name+" — "+err.Error())
				return
			}
		}
	}
	notify.Info(paths.AppName, i18n.T("cfg_deleted")+name)
}

func (c *Controller) selectConfig(name string) {
	// Open with Notepad for editing, then make it the active config.
	dir, err := paths.ConfigDir()
	if err == nil {
		_ = app.OpenInNotepad(filepath.Join(dir, name))
	}

	c.App.ActiveConfig = name
	_ = config.SaveAppSettings(c.App)
	c.rewatchActiveConfig()
	c.refreshConfigMenu()
	c.refreshNodeMenu()
	if c.Core.Running() {
		c.suppressConfigWatch(2 * time.Second)
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			log.Println("switch config:", err)
			c.setStatus(false, err.Error())
			notify.Error(paths.AppName, i18n.T("start_failed")+err.Error())
		}
	}
}

func (c *Controller) startProxy() error {
	// Download core outside the lock (network).
	if err := c.ensureCoreSync(); err != nil {
		return fmt.Errorf("%s%w", i18n.T("core_download_fail"), err)
	}

	// TUN without admin: ask to elevate instead of letting sing-box die with Access denied.
	if c.App != nil && c.App.TunMode && !app.IsElevated() {
		if app.ConfirmYesNo(i18n.T("tun_elevate_title"), i18n.T("tun_elevate_body")) {
			c.elevateAndExit()
			return fmt.Errorf("%s", i18n.T("tun_need_admin"))
		}
		return fmt.Errorf("%s", i18n.T("tun_elevate_cancel"))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	userCfg, err := config.ActiveConfigPath(c.App)
	if err != nil {
		return err
	}
	if _, err := os.Stat(userCfg); err != nil {
		return fmt.Errorf("active config missing: %s", userCfg)
	}

	runtimePath, err := config.RuntimeConfigPath()
	if err != nil {
		return err
	}
	tunMode := c.App != nil && c.App.TunMode
	if err := config.PrepareRuntimeConfig(userCfg, runtimePath, c.App.DashboardPort, tunMode); err != nil {
		return err
	}

	home, err := paths.HomeDir()
	if err != nil {
		return err
	}

	if c.App != nil {
		c.Core.CorePath = c.App.CorePath
	}
	c.Core.ConfigPath = runtimePath
	c.Core.WorkDir = home

	if err := c.Core.Start(); err != nil {
		return err
	}
	c.setStatus(true, "")
	// System proxy after core is up (skip when TUN is capturing traffic)
	if c.App != nil && c.App.SystemProxy && !c.App.TunMode {
		_ = sysproxy.Enable(c.proxyAddr())
	}
	// Allow Clash API a moment then refresh node list
	go func() {
		time.Sleep(400 * time.Millisecond)
		c.refreshNodeMenu()
	}()
	return nil
}

func (c *Controller) stopProxy() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.Core.Stop()
	c.setStatus(false, "")
	_ = sysproxy.Restore()
	return err
}

func (c *Controller) applyTrayIcon(running bool) {
	// Windows: only SetIcon (full color). Avoid TemplateIcon (can look monochrome).
	on := c.Icons.On
	off := c.Icons.Off
	tun := c.Icons.Tun
	if len(off) == 0 {
		off = on
	}
	if len(on) == 0 {
		on = off
	}
	icon := off
	if running {
		// TUN mode → color logo; other proxy modes keep monochrome On.
		if c.App != nil && c.App.TunMode && len(tun) > 0 {
			icon = tun
		} else {
			icon = on
		}
	}
	if len(icon) == 0 {
		return
	}
	systray.SetIcon(icon)
}

// refreshTrayIcon updates the tray glyph from current core + TUN state.
func (c *Controller) refreshTrayIcon() {
	running := c.Core != nil && c.Core.Running()
	c.applyTrayIcon(running)
}

func (c *Controller) setStatus(running bool, errMsg string) {
	if running {
		systray.SetTooltip(i18n.TName("tooltip_running"))
		c.applyTrayIcon(true)
		if c.mProxy != nil {
			c.mProxy.SetTitle(i18n.T("stop"))
		}
		if c.mRestart != nil {
			c.mRestart.Show()
		}
		if c.mStatus != nil {
			c.mStatus.SetTitle(i18n.T("status_running"))
		}
		return
	}
	systray.SetTooltip(i18n.TName("tooltip_stopped"))
	c.applyTrayIcon(false)
	if c.mProxy != nil {
		c.mProxy.SetTitle(i18n.T("start"))
	}
	if c.mRestart != nil {
		c.mRestart.Hide()
	}
	if c.mStatus != nil {
		if errMsg != "" {
			msg := errMsg
			if len(msg) > 80 {
				msg = msg[:80] + "…"
			}
			c.mStatus.SetTitle(i18n.T("status_error") + msg)
		} else {
			c.mStatus.SetTitle(i18n.T("status_stopped"))
		}
	}
}

func (c *Controller) onExit() {
	if c.cfgWatch != nil {
		_ = c.cfgWatch.Close()
	}
	c.shutdown()
}
