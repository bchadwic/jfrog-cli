package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	coreTests "github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-cli/utils/tests"
	"github.com/jfrog/jfrog-client-go/auth"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	clientTestUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	xrayDetails *config.ServerDetails
	xrayAuth    auth.ServiceDetails
	// JFrog CLI for Xray commands
	xrayCli *tests.JfrogCli
)

func InitXrayTests() {
	initXrayCli()
}

func authenticateXray() string {
	*tests.JfrogUrl = clientUtils.AddTrailingSlashIfNeeded(*tests.JfrogUrl)
	xrayDetails = &config.ServerDetails{XrayUrl: *tests.JfrogUrl + tests.XrayEndpoint}
	cred := fmt.Sprintf("--url=%s", xrayDetails.XrayUrl)
	if *tests.JfrogAccessToken != "" {
		xrayDetails.AccessToken = *tests.JfrogAccessToken
		cred += fmt.Sprintf(" --access-token=%s", xrayDetails.AccessToken)
	} else {
		xrayDetails.User = *tests.JfrogUser
		xrayDetails.Password = *tests.JfrogPassword
		cred += fmt.Sprintf(" --user=%s --password=%s", xrayDetails.User, xrayDetails.Password)
	}

	var err error
	if xrayAuth, err = xrayDetails.CreateXrayAuthConfig(); err != nil {
		coreutils.ExitOnErr(errors.New("Failed while attempting to authenticate with Xray: " + err.Error()))
	}
	xrayDetails.XrayUrl = xrayAuth.GetUrl()
	return cred
}

func initXrayCli() {
	if xrayCli != nil {
		return
	}
	cred := authenticateXray()
	xrayCli = tests.NewJfrogCli(execMain, "jfrog", cred)
}

// Tests basic binary scan by providing pattern (path to testdata binaries) and --licenses flag
// and asserts any error.
func TestXrayBinaryScanJson(t *testing.T) {
	output := testXrayBinaryScan(t, string(utils.Json))
	verifyJsonScanResults(t, output, 0, 1, 1)
}

func TestXrayBinaryScanSimpleJson(t *testing.T) {
	output := testXrayBinaryScan(t, string(utils.SimpleJson))
	verifySimpleJsonScanResults(t, output, 0, 0, 1, 1)
}

func testXrayBinaryScan(t *testing.T, format string) string {
	initXrayTest(t, commands.GraphScanMinXrayVersion)
	binariesPath := filepath.Join(filepath.FromSlash(tests.GetTestResourcesPath()), "xray", "binaries", "*")
	return xrayCli.RunCliCmdWithOutput(t, "scan", binariesPath, "--licenses", "--format="+format)
}

// Tests npm audit by providing simple npm project and asserts any error.
func TestXrayAuditNpmJson(t *testing.T) {
	output := testXrayAuditNpm(t, string(utils.Json))
	verifyJsonScanResults(t, output, 0, 1, 1)
}

func TestXrayAuditNpmSimpleJson(t *testing.T) {
	output := testXrayAuditNpm(t, string(utils.SimpleJson))
	verifySimpleJsonScanResults(t, output, 0, 0, 1, 1)
}

func testXrayAuditNpm(t *testing.T, format string) string {
	initXrayTest(t, commands.GraphScanMinXrayVersion)
	tempDirPath, createTempDirCallback := coreTests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	npmProjectPath := filepath.Join(filepath.FromSlash(tests.GetTestResourcesPath()), "xray", "npm")
	// Copy the npm project from the testdata to a temp dir
	assert.NoError(t, fileutils.CopyDir(npmProjectPath, tempDirPath, true, nil))
	prevWd := changeWD(t, tempDirPath)
	defer clientTestUtils.ChangeDirAndAssert(t, prevWd)
	// Run npm install before executing jfrog xr npm-audit
	assert.NoError(t, exec.Command("npm", "install").Run())

	return xrayCli.RunCliCmdWithOutput(t, "audit-npm", "--licenses", "--format="+format)
}

// Tests NuGet audit by providing simple NuGet project and asserts any error.
func TestXrayAuditNugetJson(t *testing.T) {
	output := testXrayAuditNuget(t, "single", string(utils.Json))
	verifyJsonScanResults(t, output, 0, 2, 0)
}

func TestXrayAuditNugetSimpleJson(t *testing.T) {
	output := testXrayAuditNuget(t, "single", string(utils.SimpleJson))
	verifySimpleJsonScanResults(t, output, 0, 0, 2, 0)
}

// Tests NuGet audit by providing a multi-project NuGet project and asserts any error.
func TestXrayAuditNugetMultiProject(t *testing.T) {
	output := testXrayAuditNuget(t, "multi", string(utils.Json))
	verifyJsonScanResults(t, output, 0, 5, 0)
}

func testXrayAuditNuget(t *testing.T, projectName, format string) string {
	initXrayTest(t, commands.GraphScanMinXrayVersion)
	tempDirPath, createTempDirCallback := coreTests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	projectPath := filepath.Join(filepath.FromSlash(tests.GetTestResourcesPath()), "xray", "nuget", projectName)

	assert.NoError(t, fileutils.CopyDir(projectPath, tempDirPath, true, nil))
	prevWd := changeWD(t, tempDirPath)
	defer clientTestUtils.ChangeDirAndAssert(t, prevWd)
	// Run NuGet restore before executing jfrog xr audit (NuGet)
	assert.NoError(t, exec.Command("nuget", "restore").Run())
	return xrayCli.RunCliCmdWithOutput(t, "audit", "--format="+format)
}

func TestXrayAuditGradleJson(t *testing.T) {
	output := testXrayAuditGradle(t, string(utils.Json))
	verifyJsonScanResults(t, output, 0, 0, 0)
}

func TestXrayAuditGradleSimpleJson(t *testing.T) {
	output := testXrayAuditGradle(t, string(utils.SimpleJson))
	verifySimpleJsonScanResults(t, output, 0, 0, 0, 0)
}

func testXrayAuditGradle(t *testing.T, format string) string {
	initXrayTest(t, commands.GraphScanMinXrayVersion)
	tempDirPath, createTempDirCallback := coreTests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	gradleProjectPath := filepath.Join(filepath.FromSlash(tests.GetTestResourcesPath()), "xray", "gradle")
	// Copy the gradle project from the testdata to a temp dir
	assert.NoError(t, fileutils.CopyDir(gradleProjectPath, tempDirPath, true, nil))
	prevWd := changeWD(t, tempDirPath)
	defer clientTestUtils.ChangeDirAndAssert(t, prevWd)

	return xrayCli.RunCliCmdWithOutput(t, "audit-gradle", "--licenses", "--format="+format)
}

func TestXrayAuditMavenJson(t *testing.T) {
	output := testXrayAuditMaven(t, string(utils.Json))
	verifyJsonScanResults(t, output, 0, 1, 1)
}

func TestXrayAuditMavenSimpleJson(t *testing.T) {
	output := testXrayAuditMaven(t, string(utils.SimpleJson))
	verifySimpleJsonScanResults(t, output, 0, 0, 1, 1)
}

func testXrayAuditMaven(t *testing.T, format string) string {
	initXrayTest(t, commands.GraphScanMinXrayVersion)
	tempDirPath, createTempDirCallback := coreTests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()
	mvnProjectPath := filepath.Join(filepath.FromSlash(tests.GetTestResourcesPath()), "xray", "maven")
	// Copy the maven project from the testdata to a temp dir
	assert.NoError(t, fileutils.CopyDir(mvnProjectPath, tempDirPath, true, nil))
	prevWd := changeWD(t, tempDirPath)
	defer clientTestUtils.ChangeDirAndAssert(t, prevWd)
	return xrayCli.RunCliCmdWithOutput(t, "audit-mvn", "--licenses", "--format="+format)
}

func initXrayTest(t *testing.T, minVersion string) {
	if !*tests.TestXray {
		t.Skip("Skipping Xray test. To run Xray test add the '-test.xray=true' option.")
	}
	validateXrayVersion(t, minVersion)
}

func validateXrayVersion(t *testing.T, minVersion string) {
	xrayVersion, err := getXrayVersion()
	if err != nil {
		assert.NoError(t, err)
		return
	}
	err = commands.ValidateXrayMinimumVersion(xrayVersion.GetVersion(), minVersion)
	if err != nil {
		t.Skip(err)
	}
}

func getXrayVersion() (version.Version, error) {
	xrayVersion, err := xrayAuth.GetVersion()
	return *version.NewVersion(xrayVersion), err
}

func verifyJsonScanResults(t *testing.T, content string, minViolations, minVulnerabilities, minLicenses int) {
	var results []services.ScanResponse
	err := json.Unmarshal([]byte(content), &results)
	assert.NoError(t, err)
	assert.True(t, len(results[0].Violations) >= minViolations, fmt.Sprintf("Expected at least %d violations in scan results, but got %d violations.", minViolations, len(results[0].Violations)))
	assert.True(t, len(results[0].Vulnerabilities) >= minVulnerabilities, fmt.Sprintf("Expected at least %d vulnerabilities in scan results, but got %d vulnerabilities.", minVulnerabilities, len(results[0].Vulnerabilities)))
	assert.True(t, len(results[0].Licenses) >= minLicenses, fmt.Sprintf("Expected at least %d Licenses in scan results, but got %d Licenses.", minLicenses, len(results[0].Licenses)))
}

func verifySimpleJsonScanResults(t *testing.T, content string, minSecViolations, minLicViolations, minVulnerabilities, minLicenses int) {
	var results utils.ResultsSimpleJson
	err := json.Unmarshal([]byte(content), &results)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(results.SecurityViolations), minSecViolations)
	assert.GreaterOrEqual(t, len(results.LicensesViolations), minLicViolations)
	assert.GreaterOrEqual(t, len(results.Vulnerabilities), minVulnerabilities)
	assert.GreaterOrEqual(t, len(results.Licenses), minLicenses)
}
