package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/log"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/plugin"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin"
	"github.com/twpayne/go-vfs"
)

const (
	cloudInitFile           = "cloud-init.yaml"
	installFile             = "install.yaml"
	resetFile               = "reset.yaml"
	sentinelFileResetNeeded = "reset.needed"
)

var ErrUnmanagedOSNotReset = errors.New("unmanaged OS reset sentinel file still exists")

type DummyPlugin struct {
	fs          vfs.FS
	hostManager host.Manager
	workDir     string
	configPath  string
}

func GetPlugin() (osplugin.Plugin, error) {
	return &DummyPlugin{
		fs:          vfs.OSFS,
		hostManager: host.NewManager(),
	}, nil
}

func (p *DummyPlugin) Init(context osplugin.PluginContext) error {
	if context.Debug {
		log.EnableDebug()
	}
	log.Debug("Initing Dummy Plugin")
	p.workDir = context.WorkDir
	p.configPath = context.ConfigPath
	if err := utils.CreateDirectory(p.fs, filepath.Dir(p.configPath)); err != nil {
		return fmt.Errorf("creating config directory '%s': %w", filepath.Dir(p.configPath), err)
	}
	if err := utils.CreateDirectory(p.fs, p.workDir); err != nil {
		return fmt.Errorf("creating work directory '%s': %w", p.workDir, err)
	}
	return nil
}

func (p *DummyPlugin) ApplyCloudInit(input []byte) error {
	path := fmt.Sprintf("%s/%s", p.workDir, cloudInitFile)
	cloudInitBytes := []byte("#cloud-config\n")
	cloudInitContentBytes, err := plugin.UnmarshalRawJSONToYaml(input)
	if err != nil {
		return fmt.Errorf("unmarshalling cloud init config: %w", err)
	}
	cloudInitBytes = append(cloudInitBytes, cloudInitContentBytes...)
	if err := p.fs.WriteFile(path, cloudInitBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing cloud init config: %w", err)
	}
	return nil
}

func (p *DummyPlugin) GetHostname() (string, error) {
	hostname, err := p.hostManager.GetCurrentHostname()
	if err != nil {
		return "", fmt.Errorf("getting current hostname: %w", err)
	}
	return hostname, nil
}

func (p *DummyPlugin) PersistHostname(hostname string) error {
	log.Debugf("Setting hostname %s", hostname)
	if err := p.hostManager.SetHostname(hostname); err != nil {
		return fmt.Errorf("setting hostname '%s': %w", hostname, err)
	}
	return nil
}

func (p *DummyPlugin) PersistFile(content []byte, path string, _ uint32, _ int, _ int) error {
	log.Debugf("Writing file %s", path)
	if err := utils.WriteFile(p.fs, api.WriteFile{
		Path:    path,
		Content: string(content),
	}); err != nil {
		return fmt.Errorf("writing file '%s': %w", path, err)
	}
	return nil
}

func (p *DummyPlugin) Install(input []byte) error {
	path := fmt.Sprintf("%s/%s", p.workDir, installFile)
	log.Debugf("Copying install config to file: %s", path)
	installBytes, err := plugin.UnmarshalRawJSONToYaml(input)
	if err != nil {
		return fmt.Errorf("unmarshalling install config: %w", err)
	}
	if err := p.fs.WriteFile(path, installBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing install config: %w", err)
	}
	return nil
}

func (p *DummyPlugin) TriggerReset() error {
	log.Debug("Triggering Unmanaged OS reset")
	sentinelFile := p.resetSentinelFilePath()
	log.Infof("Creating reset sentinel file: %s", sentinelFile)
	if err := p.fs.WriteFile(sentinelFile, []byte("1\n"), os.ModePerm); err != nil {
		return fmt.Errorf("writing install config: %w", err)
	}
	return nil
}

func (p *DummyPlugin) Reset(input []byte) error {
	path := fmt.Sprintf("%s/%s", p.workDir, resetFile)
	log.Debugf("Copying reset config to file: %s", path)
	resetBytes, err := plugin.UnmarshalRawJSONToYaml(input)
	if err != nil {
		return fmt.Errorf("unmarshalling reset config: %w", err)
	}
	if err := p.fs.WriteFile(path, resetBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing reset config: %w", err)
	}
	// Check reset sentinel file
	sentinelFile := p.resetSentinelFilePath()
	log.Infof("Verifying reset sentinel file '%s' has been deleted", sentinelFile)
	_, err = p.fs.Stat(sentinelFile)
	if err == nil {
		return ErrUnmanagedOSNotReset
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("getting info for file '%s': %w", sentinelFile, err)
	}
	return nil
}

func (p *DummyPlugin) PowerOff() error {
	if err := p.hostManager.PowerOff(); err != nil {
		return fmt.Errorf("powering off system: %w", err)
	}
	return nil
}

func (p *DummyPlugin) Reboot() error {
	if err := p.hostManager.Reboot(); err != nil {
		return fmt.Errorf("rebooting system: %w", err)
	}
	return nil
}

func (p *DummyPlugin) resetSentinelFilePath() string {
	return fmt.Sprintf("%s/%s", p.workDir, sentinelFileResetNeeded)
}
