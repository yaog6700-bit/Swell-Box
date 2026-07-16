package tray

import (
	"log"
	"strings"
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
	group := c.selectorTag
	if group == "" {
		group = "proxy"
	}

	// Prefer live Clash API (no full restart).
	cli := clashapi.New(clashapi.DefaultAddr)
	if err := cli.Select(group, tag); err != nil {
		log.Println("clash select:", err)
		// Fallback: write default + restart
		path, err2 := config.ActiveConfigPath(c.App)
		if err2 != nil {
			notify.Error(paths.AppName, i18n.T("node_switch_fail")+err.Error())
			return
		}
		_ = config.SetSelectorDefault(path, group, tag)
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

// refreshNodeMenu rebuilds visible node entries from config / Clash API.
func (c *Controller) refreshNodeMenu() {
	if c.mNodes == nil {
		return
	}
	var members []string
	var now string
	group := "proxy"

	// Prefer config file for the delete list (source of truth).
	// Live API only for current selection checkmark when running.
	path, err := config.ActiveConfigPath(c.App)
	if err == nil {
		sels, err := config.ListSelectors(path)
		if err == nil {
			for _, s := range sels {
				if s.Tag == "proxy" || group == s.Tag {
					group = s.Tag
					members = s.Outbounds
					now = s.Default
					break
				}
			}
			if len(members) == 0 && len(sels) > 0 {
				group = sels[0].Tag
				members = sels[0].Outbounds
				now = sels[0].Default
			}
		}
	}
	// Overlay live "now" from Clash API when running.
	if c.Core != nil && c.Core.Running() {
		cli := clashapi.New(clashapi.DefaultAddr)
		if n, all, err := cli.GroupNow(group); err == nil {
			if n != "" {
				now = n
			}
			// If config list empty but API has members, use API list for display.
			if len(members) == 0 && len(all) > 0 {
				members = all
			}
		}
	}

	// filter junk
	var clean []string
	for _, m := range members {
		low := strings.ToLower(m)
		if low == "direct" || low == "block" || low == "dns" || low == "reject" {
			continue
		}
		clean = append(clean, m)
	}
	members = clean
	c.selectorTag = group

	if len(members) == 0 {
		if c.mNodesEmpty != nil {
			c.mNodesEmpty.SetTitle(i18n.T("nodes_empty"))
			c.mNodesEmpty.Show()
		}
		for i, mi := range c.nodeSlots {
			c.nodeTags[i] = ""
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
			mi.Hide()
			continue
		}
		tag := members[i]
		c.nodeTags[i] = tag
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
	// Parent is the top-level 订阅 group (mSubs), not 添加.
	parent := c.mSubs
	if parent == nil {
		parent = c.mAdd
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
	// Resolve display name for toast before removal.
	name := rawURL
	if items, err := config.LoadSubscriptions(); err == nil {
		for _, it := range items {
			if it.URL == rawURL {
				if it.Name != "" {
					name = it.Name
				}
				break
			}
		}
	}
	if err := config.RemoveSubscription(rawURL); err != nil {
		notify.Error(paths.AppName, i18n.T("sub_delete_fail")+err.Error())
		return
	}
	c.refreshSubDeleteMenu()
	// Only removes the saved URL. Imported nodes stay until deleted under 节点 → 删除节点.
	notify.Info(paths.AppName, i18n.T("sub_deleted")+name)
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
