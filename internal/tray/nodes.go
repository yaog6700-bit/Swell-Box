package tray

import (
	"log"
	"strings"

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

// refreshNodeMenu rebuilds visible node entries from config / Clash API.
func (c *Controller) refreshNodeMenu() {
	if c.mNodes == nil {
		return
	}
	var members []string
	var now string
	group := "proxy"

	// Try live API first when running
	if c.Core != nil && c.Core.Running() {
		cli := clashapi.New(clashapi.DefaultAddr)
		if n, all, err := cli.GroupNow(group); err == nil && len(all) > 0 {
			now, members = n, all
		}
	}
	if len(members) == 0 {
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
}
