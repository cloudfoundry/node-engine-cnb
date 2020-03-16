package node

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/packit/postal"
	"github.com/cloudfoundry/packit/scribe"
)

type BuildpackMetadataDependency struct {
	ID      string   `toml:"id"`
	Name    string   `toml:"name"`
	SHA256  string   `toml:"sha256"`
	Stacks  []string `toml:"stacks"`
	URI     string   `toml:"uri"`
	Version string   `toml:"version"`
}

//go:generate faux --interface EntryResolver --output fakes/entry_resolver.go
type EntryResolver interface {
	Resolve([]packit.BuildpackPlanEntry) packit.BuildpackPlanEntry
}

//go:generate faux --interface DependencyManager --output fakes/dependency_manager.go
type DependencyManager interface {
	Resolve(path, id, version, stack string) (postal.Dependency, error)
	Install(dependency postal.Dependency, cnbPath, layerPath string) error
}

//go:generate faux --interface EnvironmentConfiguration --output fakes/environment_configuration.go
type EnvironmentConfiguration interface {
	Configure(env EnvironmentVariables, path string, optimizeMemory bool) error
}

//go:generate faux --interface BuildPlanRefinery --output fakes/build_plan_refinery.go
type BuildPlanRefinery interface {
	BillOfMaterial(dependency postal.Dependency) packit.BuildpackPlan
}

//go:generate faux --interface CacheManager --output fakes/cache_manager.go
type CacheManager interface {
	Match(layer packit.Layer, dependency BuildpackMetadataDependency) (bool, error)
}

func Build(entries EntryResolver, dependencies DependencyManager, environment EnvironmentConfiguration, planRefinery BuildPlanRefinery, cacheManager CacheManager, logger scribe.Logger, clock Clock) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)
		logger.Process("Resolving Node Engine version")

		entry := entries.Resolve(context.Plan.Entries)

		dependency, err := dependencies.Resolve(filepath.Join(context.CNBPath, "buildpack.toml"), entry.Name, entry.Version, context.Stack)
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Subprocess("Selected Node Engine version (using %s): %s", entry.Metadata["version-source"], dependency.Version)
		logger.Break()

		nodeLayer, err := context.Layers.Get(Node, packit.LaunchLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		nodeLayer.Build = entry.Metadata["build"] == true
		nodeLayer.Cache = entry.Metadata["build"] == true

		match, err := cacheManager.Match(nodeLayer, BuildpackMetadataDependency{SHA256: dependency.SHA256})
		if err != nil {
			return packit.BuildResult{}, err
		}

		bom := planRefinery.BillOfMaterial(postal.Dependency{
			ID:      dependency.ID,
			Name:    dependency.Name,
			SHA256:  dependency.SHA256,
			Stacks:  dependency.Stacks,
			URI:     dependency.URI,
			Version: dependency.Version,
		})

		if match {
			logger.Process("Reusing cached layer %s", nodeLayer.Path)
			logger.Break()

			return packit.BuildResult{
				Plan:   bom,
				Layers: []packit.Layer{nodeLayer},
			}, nil
		}

		logger.Process("Executing build process")

		err = nodeLayer.Reset()
		if err != nil {
			return packit.BuildResult{}, err
		}

		nodeLayer.Metadata = map[string]interface{}{
			DepKey:     dependency.SHA256,
			"built_at": clock.Now().Format(time.RFC3339Nano),
		}

		logger.Subprocess("Installing Node Engine %s", dependency.Version)
		then := clock.Now()
		err = dependencies.Install(dependency, context.CNBPath, nodeLayer.Path)
		if err != nil {
			return packit.BuildResult{}, err
		}
		logger.Action("Completed in %s", time.Since(then).Round(time.Millisecond))
		logger.Break()

		config, err := BuildpackYMLParser{}.Parse(filepath.Join(context.WorkingDir, "buildpack.yml"))
		if err != nil {
			return packit.BuildResult{}, fmt.Errorf("unable to parse buildpack.yml file: %s", err)
		}

		err = environment.Configure(nodeLayer.SharedEnv, nodeLayer.Path, config.OptimizedMemory)
		if err != nil {
			return packit.BuildResult{}, err
		}

		return packit.BuildResult{
			Plan:   bom,
			Layers: []packit.Layer{nodeLayer},
		}, nil
	}
}