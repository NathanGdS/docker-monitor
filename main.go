package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/NathanGdS/docker-monitor/models"
	"github.com/NathanGdS/docker-monitor/utils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type containerUpdateMsg struct {
	Running []string
	Paused  []string
	Stopped []string
}

type model struct {
	client   *client.Client
	spinner  spinner.Model
	running  []string
	paused   []string
	stopped  []string
	quitting bool
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchContainerData(m.client))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "ctrl+d":
			m.quitting = true
			return m, tea.Quit
		}

	case containerUpdateMsg:
		m.running = msg.Running
		m.paused = msg.Paused
		m.stopped = msg.Stopped
		return m, fetchContainerData(m.client)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	s := "\n" + m.spinner.View() + utils.Reset + " Monitoring Containers " + m.spinner.View() + "\n\n" + utils.Reset
	s += formatContainerSection("Running Containers:", m.running, utils.StrGreen)
	s += formatContainerSection("Paused Containers:", m.paused, utils.StrYellow)
	s += formatContainerSection("Stopped Containers:", m.stopped, utils.StrRed)
	s += "--------------------------------------\n"
	s += utils.Reset + "Last updated: " + time.Now().Format("15:04:05") + "\n\n"
	s += "Press q to exit.\n" + utils.Reset

	return s
}

func fetchContainerData(client *client.Client) tea.Cmd {
	return func() tea.Msg {
		containers := getContainers(client)
		var running, paused, stopped []string
		var wg sync.WaitGroup

		for _, ctr := range containers {
			wg.Add(1)
			go func(c container.Summary) {
				defer wg.Done()
				stats, err := getContainerStatusData(client, c)
				if err == nil {
					categorizeContainer(stats, c, &running, &paused, &stopped)
				}
			}(ctr)
		}

		wg.Wait()
		sort.Strings(running)
		sort.Strings(paused)
		sort.Strings(stopped)
		return containerUpdateMsg{running, paused, stopped}
	}
}

func categorizeContainer(s models.StatsData, c container.Summary, running, paused, stopped *[]string) {
	cpuPercent := calculateCPUPercent(&s)
	memUsage := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Usage)/1024/1024)
	memLimit := fmt.Sprintf("%.2fMB", float64(s.MemoryStats.Limit)/1024/1024)
	status := fmt.Sprintf("Container: %s (%s) | CPU: %.2f%% | Memory: %s / %s\n", c.ID[:12], c.Image, cpuPercent, memUsage, memLimit)

	switch c.State {
	case "running":
		*running = append(*running, utils.StrGreen(status))
	case "paused":
		*paused = append(*paused, utils.StrYellow(status))
	default:
		*stopped = append(*stopped, utils.StrRed(status))
	}
}

func formatContainerSection(title string, containers []string, colorFunc func(string) string) string {
	if len(containers) == 0 {
		return colorFunc("No " + title + "\n")
	}
	s := colorFunc(title + "\n")
	for _, c := range containers {
		s += c + utils.Reset
	}
	return s
}

func calculateCPUPercent(stats *models.StatsData) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta > 0.0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

func getContainers(apiClient *client.Client) []container.Summary {
	containers, err := apiClient.ContainerList(context.Background(), container.ListOptions{
		All: true,
	})
	if err != nil {
		panic(err)
	}

	return containers
}

func getContainerStatusData(client *client.Client, container container.Summary) (models.StatsData, error) {
	ctx := context.Background()

	stats, err := client.ContainerStats(ctx, container.ID, false)
	if err != nil {
		panic(err)
	}

	data, err := io.ReadAll(stats.Body)
	if err != nil {
		stats.Body.Close()
		time.Sleep(2 * time.Second)
		return models.StatsData{}, errors.New("error reading stats data body")
	}
	stats.Body.Close()

	var s models.StatsData

	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("Error unmarshaling stats JSON: %v", err)
		time.Sleep(2 * time.Second)
		return models.StatsData{}, errors.New("error unmarshaling stats JSON")
	}
	return s, nil
}

func main() {
	utils.ClearConsole()
	client, err := client.NewClientWithOpts(client.WithVersion("1.41"), client.FromEnv)
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}
	defer client.Close()

	var spinner = spinner.New()
	spinner.Style = spinner.Style.Foreground(lipgloss.NoColor{})

	p := tea.NewProgram(model{client: client, spinner: spinner})
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}
