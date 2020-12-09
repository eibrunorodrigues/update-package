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
	askForEachPackage(&packages)
	updateRequirements(&arguments, packages)
}

func askForEachPackage(packages *[]PackageModel) {

	for index, packageItem := range *packages {
		var response bool

		if packageItem.CurrentVersion == "" {
			fmt.Print("\n\nFound package " + packageItem.Package + " without a static version on it. ")
		} else {
			fmt.Print("\n\nFound package " + packageItem.Package + " on version: " + packageItem.CurrentVersion)
		}

		if packageItem.CurrentVersion == "" {
			fmt.Print("\nWould you like to set a static version on this package? ")
			response = utils.Ask()
		} else {
			fmt.Print("\nWould you like to update it? ")
			response = utils.Ask()
		}

		if response {
			var versionOutput string
			fmt.Print("Please type the new version of " + packageItem.Package + ": ")
			_, _ = fmt.Scanln(&versionOutput)
			(*packages)[index].NewVersion = versionOutput
			continue
		}

		(*packages)[index].NewVersion = packageItem.CurrentVersion
	}
}

func updateRequirements(args *Arguments, packages []PackageModel) {
	getVersion := func(pkg string, packs []PackageModel) string {
		for _, item := range packs {
			if pkg == item.Package {
				return item.NewVersion
			}
		}
		log.Panic("Package " + pkg + " not found")
		return ""
	}

	for versionFile := range findAllFilesToUpdate(args) {
		fileData := strings.Split(versionFile[0], "\n")
		var finalFile string

		for _, data := range fileData {
			if data == "" {
				continue
			}

			if strings.Contains(data, PythonPKGVersionSetter) {
				version := strings.Split(data, PythonPKGVersionSetter)
				newVersion := getVersion(version[0], packages)

				if newVersion != "" {
					finalFile += version[0] + PythonPKGVersionSetter + newVersion + "\n"
					continue
				}
				finalFile += version[0] + "\n"
				continue
			}
			newVersion := getVersion(data, packages)

			if newVersion == "" {
				finalFile += data + "\n"
				continue
			}

			finalFile += data + PythonPKGVersionSetter + newVersion + "\n"
		}
		updateRepo(args, versionFile[1], finalFile)
	}
}

func updateRepo(args *Arguments, privateRequirementsPath string, finalFile string) {

	r, err := git.PlainOpen(strings.Replace(privateRequirementsPath, args.FileName, "", -1))
	if err != nil {
		log.Panic("Error while opening local git repository " + err.Error())
	}

	w, err := r.Worktree()
	if err != nil {
		log.Panic(err.Error())
	}

	out := strings.TrimRight(privateRequirementsPath, "/" + args.FileName)
	currentPackage := out[strings.LastIndex(out, "/")+1:]

	for _, branch := range [2]string{"develop", "release"} {

		fmt.Print("\nDo you want to update the branch: " + branch + " of " + currentPackage + "? ")
		if utils.Ask() {
			if err = w.Reset(&git.ResetOptions{Mode: git.ResetMode(git.HardReset)}); err != nil {
				log.Panic(err.Error())
			}

			if err = w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch)}); err != nil {
				log.Panic(err.Error())
			}

			if err = w.Pull(&git.PullOptions{RemoteName: "origin"}); err != nil && !strings.Contains(err.Error(), "up-to-date") {
				log.Panic(err.Error())
			}

			if err := ioutil.WriteFile(privateRequirementsPath, []byte(finalFile), 0755); err != nil {
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
	flag.StringVar(&branches, "branch", "develop,release", "Branches to update splitted with commas")
	flag.StringVar(&args.Language, "lang", "python", "The programming language you would like to update the packages")
	flag.Parse()

	args.Branches = strings.Split(branches, ",")
}

func main() {
	fmt.Println("Starting to update packages")
	initApplication()
	fmt.Println("Ending Process")
}
