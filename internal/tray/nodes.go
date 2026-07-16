package tray

import (
	"fmt"
	"log"
	"time"

	"github.com/getlantern/systray"

	"github.com/swell-app/swellbox/internal/clashapi"
	"github.com/swell-app/swellbox/internal/config"
	"github.com/swell-app/swellbox/internal/i18n"
	"github.com/swell-app/swellbox/internal/notify"
	"github.com/swell-app/swellbox/internal/paths"
)

const maxNodeSlots = 36

func (c *Controller) initNodeSlots() {
	c.mNodes = systray.AddMenuItem(i18n.T("menu_nodes"), "")
	c.nodeSlots = make([]*systray.MenuItem, maxNodeSlots)
	c.nodeTags = make([]string, maxNodeSlots)
	c.nodeGroups = make([]string, maxNodeSlots)
	for i := 0; i < maxNodeSlots; i++ {
		mi := c.mNodes.AddSubMenuItemCheckbox("—", "", false)
		mi.Hide()
		c.nodeSlots[i] = mi
		idx := i
		go func() {
			for range mi.ClickedCh {
				c.onNodeClick(idx)
			}
		}()
	}
	c.mNodesEmpty = c.mNodes.AddSubMenuItem(i18n.T("nodes_empty"), "")
	c.mNodesEmpty.Disable()

	// 导入节点（剪贴板）— 与订阅分组方式一致，放在「节点」里
	c.mImport = c.mNodes.AddSubMenuItem(i18n.T("import_clipboard"), "")

	// Tray menus do not support true right-click on items; use a delete submenu.
	c.mDeleteNodes = c.mNodes.AddSubMenuItem(i18n.T("menu_delete_node"), "")
	c.deleteSlots = make([]*systray.MenuItem, maxNodeSlots)
	c.deleteTags = make([]string, maxNodeSlots)
	for i := 0; i < maxNodeSlots; i++ {
		mi := c.mDeleteNodes.AddSubMenuItem("—", "")
		mi.Hide()
		c.deleteSlots[i] = mi
		idx := i
		go func() {
			for range mi.ClickedCh {
				c.onNodeDelete(idx)
			}
		}()
	}
	c.mDeleteEmpty = c.mDeleteNodes.AddSubMenuItem(i18n.T("nodes_empty"), "")
	c.mDeleteEmpty.Disable()
}

func (c *Controller) onNodeClick(idx int) {
	if idx < 0 || idx >= len(c.nodeTags) {
		return
	}
	tag := c.nodeTags[idx]
	if tag == "" {
		return
	}
	group := c.nodeGroups[idx]
	if group == "" {
		group = c.selectorTag
	}
	if group == "" {
		group = "proxy"
	}
	primary := c.selectorTag
	if primary == "" {
		primary = "proxy"
	}

	// Prefer live Clash API (no full restart).
	cli := clashapi.New(clashapi.DefaultAddr)
	// 1) Select leaf inside its group (e.g. Singapore urltest → 🚜🇸🇬 -FREE)
	err := cli.Select(group, tag)
	if err != nil {
		log.Println("clash select", group, tag, err)
	}
	// 2) If leaf sits under a region group, also point top selector at that region
	//    (Manual → Singapore) so traffic actually uses it.
	if primary != "" && primary != group {
		if err2 := cli.Select(primary, group); err2 != nil {
			log.Println("clash select primary", primary, group, err2)
			// try selecting leaf directly on primary (simple proxy configs)
			_ = cli.Select(primary, tag)
		}
	}

	if err != nil {
		// Fallback: write default + restart
		path, err2 := config.ActiveConfigPath(c.App)
		if err2 != nil {
			notify.Error(paths.AppName, i18n.T("node_switch_fail")+err.Error())
			return
		}
		_ = config.SetSelectorDefault(path, group, tag)
		if primary != group {
			_ = config.SetSelectorDefault(path, primary, group)
		}
		if c.Core.Running() {
			_ = c.stopProxy()
			if err3 := c.startProxy(); err3 != nil {
				notify.Error(paths.AppName, i18n.T("node_switch_fail")+err3.Error())
				return
			}
		}
	} else {
		// Persist choice to user config
		if path, err := config.ActiveConfigPath(c.App); err == nil {
			_ = config.SetSelectorDefault(path, group, tag)
			if primary != group {
				_ = config.SetSelectorDefault(path, primary, group)
			}
		}
	}

	for i, mi := range c.nodeSlots {
		if mi == nil {
			continue
		}
		if c.nodeTags[i] == tag {
			mi.Check()
		} else {
			mi.Uncheck()
		}
	}
	notify.Info(paths.AppName, i18n.T("node_switched")+tag)
}

func (c *Controller) onNodeDelete(idx int) {
	if idx < 0 || idx >= len(c.deleteTags) {
		return
	}
	tag := c.deleteTags[idx]
	if tag == "" {
		return
	}
	c.suppressConfigWatch(3 * time.Second)
	if err := config.RemoveNodeFromConfig(c.App, tag); err != nil {
		notify.Error(paths.AppName, i18n.T("node_delete_fail")+err.Error())
		return
	}
	c.refreshNodeMenu()
	// Reload core so the bad/removed outbound is gone from runtime.
	if c.Core != nil && c.Core.Running() {
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			notify.Error(paths.AppName, i18n.T("node_deleted")+tag+" — "+err.Error())
			return
		}
		notify.Info(paths.AppName, i18n.T("node_deleted")+tag+i18n.T("imported_restart"))
		return
	}
	notify.Info(paths.AppName, i18n.T("node_deleted")+tag)
}

// refreshNodeMenu rebuilds visible node entries for the home profile only.
func (c *Controller) refreshNodeMenu() {
	if c.mNodes == nil {
		return
	}
	var (
		members []string
		groups  []string // parallel: clash group for each member
		now     string
		primary = "proxy"
	)

	// Only default config.json uses the 节点 menu; other profiles → Dashboard.
	homeProfile := c.App != nil && config.IsDefaultConfigName(c.App.ActiveConfig)

	path, err := config.ActiveConfigPath(c.App)
	if err == nil && homeProfile {
		p, nodes, cur, err := config.ListSwitchableNodes(path)
		if err == nil {
			if p != "" {
				primary = p
			}
			now = cur
			for _, n := range nodes {
				members = append(members, n.Tag)
				groups = append(groups, n.Group)
			}
		}
	}

	// Overlay live "now" from Clash API when running (home profile only).
	if homeProfile && c.Core != nil && c.Core.Running() && primary != "" {
		cli := clashapi.New(clashapi.DefaultAddr)
		if n, _, err := cli.GroupNow(primary); err == nil && n != "" {
			now = n
		}
	}

	c.selectorTag = primary

	if len(members) == 0 {
		if c.mNodesEmpty != nil {
			emptyMsg := i18n.T("nodes_empty")
			if !homeProfile {
				emptyMsg = i18n.T("nodes_use_dashboard")
			}
			c.mNodesEmpty.SetTitle(emptyMsg)
			c.mNodesEmpty.Show()
		}
		for i, mi := range c.nodeSlots {
			c.nodeTags[i] = ""
			if i < len(c.nodeGroups) {
				c.nodeGroups[i] = ""
			}
			if mi != nil {
				mi.Hide()
			}
		}
		c.fillDeleteSlots(nil)
		return
	}
	if c.mNodesEmpty != nil {
		c.mNodesEmpty.Hide()
	}

	for i := 0; i < maxNodeSlots; i++ {
		mi := c.nodeSlots[i]
		if mi == nil {
			continue
		}
		if i >= len(members) {
			c.nodeTags[i] = ""
			if i < len(c.nodeGroups) {
				c.nodeGroups[i] = ""
			}
			mi.Hide()
			continue
		}
		tag := members[i]
		c.nodeTags[i] = tag
		if i < len(groups) {
			c.nodeGroups[i] = groups[i]
		} else {
			c.nodeGroups[i] = primary
		}
		mi.SetTitle(tag)
		mi.Show()
		if tag == now {
			mi.Check()
		} else {
			mi.Uncheck()
		}
	}
	c.fillDeleteSlots(members)
}

const maxSubSlots = 16

func (c *Controller) initSubDeleteSlots() {
	parent := c.mSubs
	if parent == nil {
		return
	}
	c.mDeleteSubs = parent.AddSubMenuItem(i18n.T("menu_delete_sub"), "")
	c.subDelSlots = make([]*systray.MenuItem, maxSubSlots)
	c.subDelURLs = make([]string, maxSubSlots)
	for i := 0; i < maxSubSlots; i++ {
		mi := c.mDeleteSubs.AddSubMenuItem("—", "")
		mi.Hide()
		c.subDelSlots[i] = mi
		idx := i
		go func() {
			for range mi.ClickedCh {
				c.onSubDelete(idx)
			}
		}()
	}
	c.mDeleteSubEmpty = c.mDeleteSubs.AddSubMenuItem(i18n.T("sub_none"), "")
	c.mDeleteSubEmpty.Disable()
}

func (c *Controller) onSubDelete(idx int) {
	if idx < 0 || idx >= len(c.subDelURLs) {
		return
	}
	rawURL := c.subDelURLs[idx]
	if rawURL == "" {
		return
	}
	c.suppressConfigWatch(3 * time.Second)

	removed, err := config.RemoveSubscription(rawURL)
	if err != nil {
		notify.Error(paths.AppName, i18n.T("sub_delete_fail")+err.Error())
		return
	}
	name := rawURL
	if removed != nil && removed.Name != "" {
		name = removed.Name
	}

	// Also drop nodes that were imported from this subscription.
	nodeN := 0
	if removed != nil && len(removed.NodeTags) > 0 {
		if n, err := config.RemoveNodesFromConfig(c.App, removed.NodeTags); err == nil {
			nodeN = n
		}
	}

	c.refreshSubDeleteMenu()
	c.refreshNodeMenu()
	if c.Core != nil && c.Core.Running() && nodeN > 0 {
		_ = c.stopProxy()
		if err := c.startProxy(); err != nil {
			notify.Error(paths.AppName, fmt.Sprintf(i18n.T("sub_deleted_nodes"), name, nodeN)+" — "+err.Error())
			return
		}
	}
	if nodeN > 0 {
		notify.Info(paths.AppName, fmt.Sprintf(i18n.T("sub_deleted_nodes"), name, nodeN))
	} else {
		// Older subs (before tag tracking) have no node_tags — only the URL is removed.
		notify.Info(paths.AppName, i18n.T("sub_deleted")+name+i18n.T("sub_deleted_no_tags"))
	}
}

func (c *Controller) refreshSubDeleteMenu() {
	if c.mDeleteSubs == nil {
		return
	}
	items, err := config.LoadSubscriptions()
	if err != nil {
		items = nil
	}
	if len(items) == 0 {
		if c.mDeleteSubEmpty != nil {
			c.mDeleteSubEmpty.SetTitle(i18n.T("sub_none"))
			c.mDeleteSubEmpty.Show()
		}
		for i, mi := range c.subDelSlots {
			c.subDelURLs[i] = ""
			if mi != nil {
				mi.Hide()
			}
		}
		return
	}
	if c.mDeleteSubEmpty != nil {
		c.mDeleteSubEmpty.Hide()
	}
	for i := 0; i < maxSubSlots; i++ {
		mi := c.subDelSlots[i]
		if mi == nil {
			continue
		}
		if i >= len(items) {
			c.subDelURLs[i] = ""
			mi.Hide()
			continue
		}
		it := items[i]
		c.subDelURLs[i] = it.URL
		title := it.Name
		if title == "" {
			title = it.URL
		}
		// Cap very long titles for the menu.
		if r := []rune(title); len(r) > 40 {
			title = string(r[:40]) + "…"
		}
		mi.SetTitle(title)
		mi.Show()
	}
}

func (c *Controller) fillDeleteSlots(members []string) {
	if c.mDeleteNodes == nil {
		return
	}
	if len(members) == 0 {
		if c.mDeleteEmpty != nil {
			c.mDeleteEmpty.SetTitle(i18n.T("nodes_empty"))
			c.mDeleteEmpty.Show()
		}
		for i, mi := range c.deleteSlots {
			c.deleteTags[i] = ""
			if mi != nil {
				mi.Hide()
			}
		}
		return
	}
	if c.mDeleteEmpty != nil {
		c.mDeleteEmpty.Hide()
	}
	for i := 0; i < maxNodeSlots; i++ {
		mi := c.deleteSlots[i]
		if mi == nil {
			continue
		}
		if i >= len(members) {
			c.deleteTags[i] = ""
			mi.Hide()
			continue
		}
		tag := members[i]
		c.deleteTags[i] = tag
		mi.SetTitle(tag)
		mi.Show()
	}
}
