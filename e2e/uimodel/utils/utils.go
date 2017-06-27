package utils

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/robotest/e2e/runtime"
	"github.com/gravitational/robotest/e2e/uimodel/defaults"

	. "github.com/onsi/gomega"
	web "github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"
)

// IsErrorPage checks if error page is open
func IsErrorPage(page *web.Page) bool {
	return IsFound(page, ".grv-msg-page")
}

// IsErrorPage checks if installer page is open
func IsInstaller(page *web.Page) bool {
	return IsFound(page, ".grv-installer")
}

// HasValidationErrors checks if validation errors are present
func HasValidationErrors(page *web.Page) bool {
	return IsFound(page, "label.error")
}

// IsLoginPage checks if login page is open
func IsLoginPage(page *web.Page) bool {
	count, _ := page.FindByClass("grv-user-login").Count()
	return count > 0
}

// IsFound checks if element selector matches any elements
func IsFound(page *web.Page, className string) bool {
	el := page.Find(className)
	count, _ := el.Count()
	return count > 0
}

// FormatUrl returns URL by appending given prefix to page base URL
func FormatUrl(page *web.Page, prefix string) string {
	url, err := page.URL()
	Expect(err).NotTo(HaveOccurred())
	return URLPathFromString(url, prefix)
}

// GetSiteURL returns cluster page URL
func GetSiteURL(page *web.Page, clusterName string) string {
	urlPrefix := fmt.Sprintf("/web/site/%v", clusterName)
	return FormatUrl(page, urlPrefix)
}

// GetOpsCenterURL returns Ops Center page URL
func GetOpsCenterURL(page *web.Page) string {
	return FormatUrl(page, "/web/portal")
}

// GetSiteServersURL returns cluster serve page URL
func GetSiteServersURL(page *web.Page, clusterName string) string {
	clusterURL := GetSiteURL(page, clusterName)
	return fmt.Sprintf("%v/servers", clusterURL)
}

// FillOutAWSKeys fills out AWS access and secret fields with given values
func FillOutAWSKeys(page *web.Page, accessKey string, secretKey string) {
	Expect(page.FindByName("aws_access_key").Fill(accessKey)).To(Succeed(), "should enter access key")
	Expect(page.FindByName("aws_secret_key").Fill(secretKey)).To(Succeed(), "should enter secret key")
}

// SetDropdownValue sets a value to dropdown element
func SetDropdownValue(page *web.Page, classPath string, value string) {
	const scriptTemplate = `
            var result = [];
            var cssSelector = "%v .Select-option";
            var children = document.querySelectorAll(cssSelector);
            children.forEach( z => result.push(z.innerText) );
            return result; `

	if !strings.HasPrefix(classPath, ".") {
		classPath = "." + classPath
	}

	var result []string
	page.Find(classPath).Click()
	PauseForComponentJs()
	script := fmt.Sprintf(scriptTemplate, classPath)
	page.RunScript(script, nil, &result)
	for i, optionValue := range result {
		if optionValue == value {
			optionClass := fmt.Sprintf("%v .Select-option:nth-child(%v)", classPath, i+1)
			Expect(page.Find(optionClass)).To(BeFound())
			Expect(page.Find(optionClass).Click()).To(Succeed())
			PauseForComponentJs()
			return
		}
	}

	runtime.Failf("failed to select value %q in dropdown %q", value, classPath)
}

// SetDropdownValue2 sets a value to dropdown element
// There are 2 different controls that UI uses for dropdown thus each requires different handling
func SetDropdownValue2(page *web.Page, rootSelector, buttonSelector, value string) {
	var options []string
	const scriptTemplate = `
            var options = [];
            var cssSelector = "%v .dropdown-menu a";
            var children = document.querySelectorAll(cssSelector);
            children.forEach( option => options.push(option.innerText) );
            return options;
        `

	if buttonSelector == "" {
		buttonSelector = rootSelector
	} else {
		buttonSelector = fmt.Sprintf("%v %v", rootSelector, buttonSelector)
	}

	Expect(page.Find(buttonSelector).Click()).To(Succeed())
	PauseForComponentJs()
	script := fmt.Sprintf(scriptTemplate, rootSelector)
	Expect(page.RunScript(script, nil, &options)).To(Succeed())
	for index, optionValue := range options {
		if optionValue == value {
			optionClass := fmt.Sprintf("%v li:nth-child(%v) a", rootSelector, index+1)
			Expect(page.Find(optionClass)).To(BeFound())
			Expect(page.Find(optionClass).Click()).To(Succeed(), "should select given dropdown value")
			PauseForComponentJs()
			return
		}
	}

	runtime.Failf("failed to select value %q in dropdown %q", value, rootSelector)
}

// SelectRadio sets a value to select element
func SelectRadio(page *web.Page, selector string, matches valueMatcher) {
	const scriptTemplate = `
            var options = [];
            var cssSelector = "%v span";
            var children = document.querySelectorAll(cssSelector);
            children.forEach(option => options.push(option.innerText));
            return options;
        `

	script := fmt.Sprintf(scriptTemplate, selector)
	var options []string

	Expect(page.RunScript(script, nil, &options)).To(Succeed())

	for i, optionValue := range options {
		if matches(optionValue) {
			optionClass := fmt.Sprintf("%v:nth-child(%v) span", selector, i+1)
			Expect(page.Find(optionClass)).To(BeFound())
			Expect(page.Find(optionClass).Click()).To(
				Succeed(),
				"should select given radio control")
			return
		}
	}

	runtime.Failf("failed to select control in %q", selector)
}

func ParseURL(path string, message string) *url.URL {
	if message == "" {
		message = "should be valid URL"
	}
	url, err := url.Parse(path)
	Expect(len(path)).To(BeNumerically(">", 0), message)
	Expect(err).NotTo(HaveOccurred(), message)
	return url
}

func ExtractBaseURL(path string) string {
	url := ParseURL(path, "")
	return url.Host
}

func MakeClusterInstallURL(baseURL string, clusterName string) string {
	url := ExtractBaseURL(baseURL)
	return fmt.Sprintf("%v/web/installer/site/%v/", url, clusterName)
}

func MakeClusterURL(baseURL string, clusterName string) string {
	url := ExtractBaseURL(baseURL)
	return fmt.Sprintf("%v/web/site/%v/", url, clusterName)
}

// URLPathFromString returns a new URL with the specified URL urlS using path as a custom URL path
func URLPathFromString(urlS string, path string) string {
	url, err := url.Parse(urlS)
	Expect(err).NotTo(HaveOccurred())
	url.RawQuery = ""
	url.Path = path
	return url.String()
}

// valueMatcher defines an interface to select a value
type valueMatcher func(value string) bool

func PauseForPageJs() {
	time.Sleep(1 * time.Second)
}

func PauseForComponentJs() {
	time.Sleep(100 * time.Microsecond)
}

func PauseForServerListRefresh() {
	time.Sleep(defaults.SiteServerListRefreshTimeout)
}

func Pause(params ...time.Duration) {
	timeInterval := 100 * time.Millisecond
	if len(params) != 0 {
		timeInterval = params[0]
	}

	time.Sleep(timeInterval)
}
