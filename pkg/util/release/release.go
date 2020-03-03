package release

import (
	"encoding/json"
	"github.com/funceasy/funceasy-cli/pkg/util/terminal"
	"io/ioutil"
	"net/http"
)

type Release struct {
	Name string `json:"name"`
	TagName string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	Assets []Asset `json:"assets"`
}

type Asset struct {
	Name string `json:"name"`
	Download string `json:"browser_download_url"`
}

const GetReleaseUrl string = `https://api.github.com/repos/FuncEasy/FuncEasy/releases`

func GetRelease() []Release {
	t := terminal.NewTerminalPrint()
	done := make(chan bool)
	t.PrintLoadingOneLine(done, "Fetching Release: %s", GetReleaseUrl)
	res, err := http.Get(GetReleaseUrl)
	if err != nil {
		t.PrintErrorOneLineWithExit(err)
	}
	defer res.Body.Close()
	releaseByte, err := ioutil.ReadAll(res.Body)
	var releaseList []Release
	err = json.Unmarshal(releaseByte, &releaseList)
	if err != nil {
		t.PrintErrorOneLineWithExit(err)
	}
	done<-true
	t.PrintSuccessOneLine("Fetch Release: %s", GetReleaseUrl)
	t.LineEnd()
	return releaseList
}

func GetLatestRelease() Release  {
	t := terminal.NewTerminalPrint()
	done := make(chan bool)
	t.PrintLoadingOneLine(done, "Fetching Release Latest: %s", GetReleaseUrl + "/latest")
	res, err := http.Get(GetReleaseUrl + "/latest")
	if err != nil {
		t.PrintErrorOneLineWithExit(err)
	}
	defer res.Body.Close()
	releaseByte, err := ioutil.ReadAll(res.Body)
	var release Release
	err = json.Unmarshal(releaseByte, &release)
	if err != nil {
		t.PrintErrorOneLineWithExit(err)
	}
	done<-true
	t.PrintSuccessOneLine("Fetch Release Latest: %s", GetReleaseUrl + "/latest")
	t.LineEnd()
	t.PrintInfoOneLine("Latest Version: %s", release.Name)
	t.LineEnd()
	return release
}

func Download(downloadUrl string) []byte {
	t := terminal.NewTerminalPrint()
	done := make(chan bool)
	t.PrintLoadingOneLine(done, "Downloading Release...")
	res, err := http.Get(downloadUrl)
	if err != nil {
		t.PrintErrorOneLineWithExit(err)
	}
	defer res.Body.Close()
	releaseByte, err := ioutil.ReadAll(res.Body)
	done<-true
	t.PrintSuccessOneLine("Download Complete")
	t.LineEnd()
	return releaseByte
}