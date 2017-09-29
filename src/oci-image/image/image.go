package image

import (
	"fmt"
	"io"
	"oci-image/layer"
	"os"
	"path/filepath"

	"github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . LayerManager
type LayerManager interface {
	Extract(string, string, []string) error
	Delete(string) error
	State(string) (layer.State, error)
}

type Manager struct {
	srcDir       string
	outputDir    string
	manifest     v1.Manifest
	layerManager LayerManager
	output       io.Writer
}

func NewManager(srcDir, outputDir string, manifest v1.Manifest, layerManager LayerManager, output io.Writer) *Manager {
	return &Manager{
		srcDir:       srcDir,
		outputDir:    outputDir,
		manifest:     manifest,
		layerManager: layerManager,
		output:       output,
	}
}

func (m *Manager) Extract() (string, error) {
	if err := os.MkdirAll(m.outputDir, 0755); err != nil {
		return "", err
	}

	parentLayerPaths := []string{}
	for _, l := range m.manifest.Layers {
		layerId := l.Digest.Encoded()
		layerTgz := filepath.Join(m.srcDir, layerId)
		layerDir := filepath.Join(m.outputDir, layerId)

		state, err := m.layerManager.State(layerId)
		if err != nil {
			return "", err
		}

		switch state {
		case layer.Incomplete:
			if err := m.layerManager.Delete(layerId); err != nil {
				return "", err
			}
			fallthrough
		case layer.NotExist:
			if err := os.MkdirAll(layerDir, 0755); err != nil {
				return "", err
			}

			fmt.Fprintf(m.output, "Extracting %s... ", layerId)
			if err := m.layerManager.Extract(layerTgz, layerId, parentLayerPaths); err != nil {
				return "", err
			}
			fmt.Fprintln(m.output, "Done.")
		case layer.Valid:
			// do nothing, layer already exists
		default:
			panic(fmt.Sprintf("invalid layer state: %d", state))
		}

		parentLayerPaths = append([]string{layerDir}, parentLayerPaths...)
	}

	return parentLayerPaths[0], nil
}