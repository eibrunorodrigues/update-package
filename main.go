// Script for updating package versions of projects
package main

import (
	"flag"
	"fmt"
	"github.com/eibrunorodrigues/update-packages/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	PythonPKGVersionSetter = "=="
)

type PackageModel struct {
	Package        string
	CurrentVersion string
	NewVersion     string
}

type Arguments struct {
	Path     string
	FileName string
	Branches []string
	Language string
}

func initApplication() {
	var arguments Arguments
	registerArgs(&arguments)
	validateArgs(&arguments)

	if !strings.HasSuffix(arguments.Path, "/") {
		arguments.Path += "/"
	}

	var packages []PackageModel

	getPackagesRequirements(&arguments, &packages)
	updateRequirements(&arguments, &packages)
}

func fillLinePackageVersion(packages *[]PackageModel, currentMicroservice string, linePackageName string, linePackageVersion *string, currentVersion string) {
	questionHead := "\n[" + currentMicroservice + "." + linePackageName + "]"
	if currentVersion == "" {
		fmt.Print(questionHead + " version is static (which means it will get all new versions on release)")
	} else {
		fmt.Print(questionHead + " version is " + currentVersion)
	}

	if !utils.Ask(questionHead + "... Would you like to keep it?") {
		defaultPackageIdx := getPackageIdx(*packages, linePackageName)

		if !utils.Ask(questionHead + " Default version is " + (*packages)[defaultPackageIdx].NewVersion + " would you like to use it here?") {
			fmt.Print()
			versionOutput := receiveVersionInput()

			if utils.Ask(questionHead + " Do you want to keep this version as default?") {
				(*packages)[defaultPackageIdx].NewVersion = versionOutput
			}
			*linePackageVersion = versionOutput
		}

	} else {
		*linePackageVersion = currentVersion
	}
}

func updateRequirements(args *Arguments, packages *[]PackageModel) {
	for versionFile := range findAllFilesToUpdate(args) {
		currentMicroservice := getMicroserviceBasePath(args, versionFile[1])

		fileData := strings.Split(versionFile[0], "\n")
		var finalFile string

		for _, data := range fileData {
			if data == "" {
				continue
			}

			var linePackageName string
			var linePackageVersion string

			if strings.Contains(data, PythonPKGVersionSetter) {
				version := strings.Split(data, PythonPKGVersionSetter)
				linePackageName = version[0]

				fillLinePackageVersion(packages, currentMicroservice, linePackageName, &linePackageVersion, version[1])

				finalFile += linePackageName + PythonPKGVersionSetter + linePackageVersion + "\n"
			} else {
				linePackageName = data

				fillLinePackageVersion(packages, currentMicroservice, linePackageName, &linePackageVersion, "")

				if linePackageVersion == "" {
					finalFile += linePackageName + "\n"
				} else {
					finalFile += linePackageName + PythonPKGVersionSetter + linePackageVersion + "\n"
				}
			}
		}

		updateRepo(args, versionFile[1], finalFile, currentMicroservice)
	}
}

func receiveVersionInput() string {
	getMatch := func(version string) bool {
		match, _ := regexp.MatchString("^[0-9]\\.[0-9]{1,2}(\\.[0-9]{1,3})?$", version)
		return match
	}

	versionOutput := ""
	counter := 0
	for !getMatch(versionOutput) {
		if counter > 0 {
			fmt.Print("\nWrong format... Please follow the rule: 0.00.00{0} ... ")
		}
		counter++
		_, _ = fmt.Scanln(&versionOutput)
	}

	return versionOutput
}

func getMicroserviceBasePath(args *Arguments, microserviceFilePath string) string {
	out := strings.TrimRight(microserviceFilePath, "/"+args.FileName)
	currentPackage := out[strings.LastIndex(out, "/")+1:]
	return currentPackage
}

func updateRepo(args *Arguments, packagesInfoFilePath string, finalFile string, currentPackage string) {

	r, err := git.PlainOpen(strings.Replace(packagesInfoFilePath, args.FileName, "", -1))
	if err != nil {
		log.Panic("Error while opening local git repository " + err.Error())
	}

	w, err := r.Worktree()
	if err != nil {
		log.Panic(err.Error())
	}

	for _, branch := range args.Branches {

		if utils.Ask("\nDo you want to update the branch: " + branch + " of " + currentPackage + "? ") {
			if err = w.Reset(&git.ResetOptions{Mode: git.ResetMode(git.HardReset)}); err != nil {
				log.Panic(err.Error())
			}

			if err = w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch)}); err != nil {
				log.Panic(err.Error())
			}

			if err = w.Pull(&git.PullOptions{RemoteName: "origin"}); err != nil && !strings.Contains(err.Error(), "up-to-date") {
				log.Panic(err.Error())
			}

			if err := ioutil.WriteFile(packagesInfoFilePath, []byte(finalFile), 0755); err != nil {
				log.Panic("Error saving file " + err.Error())
			}

			if _, err = w.Commit("Updating Packages via script...", &git.CommitOptions{Author: &object.Signature{Name: "Bruno Rodrigues", Email: "bruno.dasilva@ambevtech.com.br", When: time.Now()}}); err != nil {
				log.Panic(err.Error())
			}

			if err = r.Push(&git.PushOptions{RemoteName: branch, RefSpecs: []config.RefSpec{"refs/remotes/origin/*"}}); err != nil {
				log.Panic(err.Error())
			}
		}
	}
}

func getPackagesRequirements(args *Arguments, packages *[]PackageModel) {
	for versionFile := range findAllFilesToUpdate(args) {
		fileData := strings.Split(versionFile[0], "\n")

		for _, data := range fileData {
			if data == "" {
				continue
			}

			if strings.Contains(data, PythonPKGVersionSetter) {
				version := strings.Split(data, PythonPKGVersionSetter)
				if !contains(*packages, version[0]) {
					*packages = append(*packages, PackageModel{Package: version[0], CurrentVersion: version[1]})
				}
				continue
			}

			if !contains(*packages, data) {
				*packages = append(*packages, PackageModel{Package: data})
			}
		}
	}
}

func findAllFilesToUpdate(args *Arguments) <-chan [2]string {
	response := make(chan [2]string)
	go func() {
		for _, match := range getAllProjects(args.FileName, args.Path) {
			result, err := ioutil.ReadFile(match)

			if err != nil {
				log.Panic(err)
			}

			response <- [2]string{string(result), match}
		}
		close(response)
	}()
	return response
}

func getAllProjects(fileName string, defaultDirectory string) []string {
	matches, err := filepath.Glob(defaultDirectory + "**/" + fileName)
	if err != nil {
		log.Panic(err)
	}

	return matches
}

func contains(packagesList []PackageModel, argument string) bool {
	for _, value := range packagesList {
		if value.Package == argument {
			return true
		}
	}
	return false
}

func getPackage(packagesList *[]PackageModel, argument string) *PackageModel {
	for _, value := range *packagesList {
		if value.Package == argument {
			return &value
		}
	}
	return &PackageModel{}
}

func getPackageIdx(packagesList []PackageModel, argument string) int {
	for index, value := range packagesList {
		if value.Package == argument {
			return index
		}
	}
	return -1
}

func validateArgs(args *Arguments) {
	if args.Path == "" {
		log.Fatalln("You should pass something")
	}

	if _, err := os.Stat(args.Path); os.IsNotExist(err) {
		log.Fatalln("The argument " + args.Path + " is not a valid folder")
	}

	for _, branch := range args.Branches {
		if branch != "develop" && branch != "release" && branch != "master" {
			log.Fatalln("Branch " + branch + " not allowed ")
		}
	}

}

func registerArgs(args *Arguments) {
	var branches string
	flag.StringVar(&args.FileName, "file", "private-requirements.txt", "Name of the File to change")
	flag.StringVar(&args.Path, "path", "~/", "Path to the Projects")
	flag.StringVar(&branches, "branch", "develop,release", "Branches to update split with commas")
	flag.StringVar(&args.Language, "lang", "python", "The programming language you would like to update the packages")
	flag.Parse()

	args.Branches = strings.Split(branches, ",")
}

func main() {
	fmt.Println("Starting to update packages")
	initApplication()
	fmt.Println("Ending Process")
}
