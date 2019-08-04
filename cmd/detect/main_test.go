package main

import (
	"fmt"
	"github.com/cloudfoundry/node-engine-cnb/node"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/libcfbuildpack/detect"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitDetect(t *testing.T) {
	spec.Run(t, "Detect", testDetect, spec.Report(report.Terminal{}))
}

func testDetect(t *testing.T, when spec.G, it spec.S) {
	var (
		factory              *test.DetectFactory
		buildpackYamlVersion = "1.2.3"
		nvmrcVersion         = "4.5.6"
		buildpackYAMLString  = fmt.Sprintf("nodejs:\n  version: %s", buildpackYamlVersion)
		buildPlan            buildplan.Plan
	)

	it.Before(func() {
		factory = test.NewDetectFactory(t)
	})

	it("always passes", func() {
		runDetectAndExpectBuildplan(factory, buildplan.Plan{}, t)
	})

	when("there is a buildpack.yml", func() {
		it.Before(func() {
			test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, "buildpack.yml"), buildpackYAMLString)
		})

		it("should request the node version in the buildpack.yml", func() {
			buildPlan = getStandardBuildplanWithNodeVersion(buildpackYamlVersion)
			runDetectAndExpectBuildplan(factory, buildPlan, t)
		})
	})

	when("there is an .nvmrc", func() {
		it.Before(func() {
			test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, ".nvmrc"), nvmrcVersion)
		})

		it("should request the node version in the .nvmrc file", func() {
			buildPlan = getStandardBuildplanWithNodeVersion(nvmrcVersion)
			runDetectAndExpectBuildplan(factory, buildPlan, t)
		})
	})

	when("there is a buildpack.yml and a .nvmrc", func() {
		it.Before(func() {
			test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, "buildpack.yml"), buildpackYAMLString)
			test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, ".nvmrc"), nvmrcVersion)
		})

		it("should request the node version in the buildpack.yml", func() {
			buildPlan = getStandardBuildplanWithNodeVersion(buildpackYamlVersion)
			runDetectAndExpectBuildplan(factory, buildPlan, t)
		})
	})

	when("there is an empty buildpack.yml and a .nvmrc", func() {
		it.Before(func() {
			test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, "buildpack.yml"), "---\n")
			test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, ".nvmrc"), nvmrcVersion)
		})

		it("should request the node version in the .nvmrc", func() {
			buildPlan = getStandardBuildplanWithNodeVersion(nvmrcVersion)
			runDetectAndExpectBuildplan(factory, buildPlan, t)
		})
	})

	when("there is a buildpack.yml with an empty version", func() {
		it.Before(func() {
			test.WriteFile(t, filepath.Join(factory.Detect.Application.Root, "buildpack.yml"), "nodejs:\n  version:  \n")
		})

		it("should not overwrite an existing version in the build plan", func() {
			runDetectAndExpectBuildplan(factory, buildplan.Plan{}, t)
		})
	})
}

func runDetectAndExpectBuildplan(factory *test.DetectFactory, buildplan buildplan.Plan, t *testing.T) {
	Expect := NewWithT(t).Expect

	code, err := runDetect(factory.Detect)
	Expect(err).NotTo(HaveOccurred())

	Expect(code).To(Equal(detect.PassStatusCode))

	Expect(factory.Plans).To(test.HavePlans(buildplan))
}

func getStandardBuildplanWithNodeVersion(version string) buildplan.Plan {
	return buildplan.Plan{
		Provides: []buildplan.Provided{
			{Name: node.Dependency},
		},
		Requires: []buildplan.Required{
			{
				Name:    node.Dependency,
				Version: version,
				Metadata: buildplan.Metadata{
					"launch": true,
				},
			},
		},
	}
}
