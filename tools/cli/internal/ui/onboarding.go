/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package ui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/thunder-id/thunderid/tools/cli/internal/commands/sample"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/config"
)

// onboardingItem is a single entry in the first-run picker.
type onboardingItem struct {
	emoji           string
	title           string
	description     string
	sampleName      string
	comingSoon      bool
	requiredConfigs []ConfigInput
	sampleEnvTarget string
	sampleFeatures  []string
}

func (i onboardingItem) FilterValue() string { return "" }

// onboardingDelegate renders two-line items; coming-soon items are dimmed.
type onboardingDelegate struct{}

func (d onboardingDelegate) Height() int                             { return 2 }
func (d onboardingDelegate) Spacing() int                            { return 1 }
func (d onboardingDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d onboardingDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(onboardingItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	if i.comingSoon {
		fmt.Fprintln(w, "    "+Dim(i.emoji+"  "+i.title)+"  "+Dim("· Coming Soon")) //nolint:errcheck
		fmt.Fprint(w, "      "+Dim(i.description))                                  //nolint:errcheck
		return
	}

	if isSelected {
		//nolint:errcheck
		fmt.Fprintln(w, "  "+brandStyle.Render("❯ ")+Bold(brandStyle.Render(i.emoji+"  "+i.title)))
		fmt.Fprint(w, "      "+i.description) //nolint:errcheck
	} else {
		fmt.Fprintln(w, "    "+i.emoji+"  "+i.title) //nolint:errcheck
		fmt.Fprint(w, "      "+Dim(i.description))   //nolint:errcheck
	}
}

const onboardingListHeight = 18

func newOnboardingList(width int) list.Model {
	var items []list.Item
	for _, u := range Usecases {
		items = append(items, onboardingItem{
			emoji:           u.Emoji,
			title:           u.Title,
			description:     u.Description,
			sampleName:      u.SampleName,
			comingSoon:      u.ComingSoon,
			requiredConfigs: u.RequiredConfigs,
			sampleEnvTarget: u.SampleEnvTarget,
			sampleFeatures:  u.SampleFeatures,
		})
	}

	l := list.New(items, onboardingDelegate{}, width, onboardingListHeight)
	l.Title = "What would you like to try?"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false) // we render a custom hint line below the list
	l.DisableQuitKeybindings()
	l.Styles.Title = lipgloss.NewStyle().Bold(true).MarginLeft(2)
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)
	return l
}

// selectOnboarding is called when the user presses Enter in the picker.
// Coming-soon items are silently ignored; available items either trigger their
// sample directly or enter the generic config-collection flow first.
func (m *ReplModel) selectOnboarding() tea.Cmd {
	item, ok := m.onboardingList.SelectedItem().(onboardingItem)
	if !ok || item.comingSoon {
		return nil
	}

	_ = config.MarkOnboardingDone(m.version)
	m.showOnboarding = false
	m.messages = nil // clear startup messages; sample progress starts on a clean slate

	if item.sampleName == "" {
		m.input.Focus()
		m.input.Placeholder = "Type / for commands, Ctrl+C to exit"
		m.messages = append(m.messages, Yellow("⏳")+" "+Bold(item.title)+" sample is coming soon.")
		return nil
	}

	if len(item.requiredConfigs) > 0 {
		return func() tea.Msg {
			return usecaseConfigRequestMsg{
				sampleName: item.sampleName,
				inputs:     item.requiredConfigs,
				envTarget:  item.sampleEnvTarget,
				features:   item.sampleFeatures,
			}
		}
	}

	m.tryingOut = true
	m.input.Blur()
	return makeTryCmd(item.sampleName, m.installPath, m.verbose, sample.Options{})
}
