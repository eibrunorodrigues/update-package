// Script for updating private-requirements of python projects
package main

import (
	"fmt"
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

type PackageModel struct {
	Package        string
	CurrentVersion string
	NewVersion     string
}

func initApplication() {
	fatalErrors := make(chan string)
	validateArgs()
	defaultDirectory := os.Args[1:][0]

	if !strings.HasSuffix(defaultDirectory, "/") {
		defaultDirectory = defaultDirectory + "/"
	}

	var packages []PackageModel

	getPackagesRequirements(&fatalErrors, defaultDirectory, &packages)
	askForEachPackage(&packages)
	updateRequirements(defaultDirectory, packages)

	for errors := range fatalErrors {
		log.Fatalln(errors)
	}
}

func ask() bool {
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	notOkayResponses := []string{"n", "N", "no", "No", "NO"}
	allAnswers := addToList(okayResponses, notOkayResponses)
	loop := 0
	var responseStr string

	for !containsString(allAnswers, responseStr) {
		if loop > 0 {
			fmt.Println("Wrong answer... Allowed responses are: y/N")
		}

		fmt.Print("\nType your answer... y/N : ")
		_, err := fmt.Scanln(&responseStr)

		if err != nil {
			log.Panic(err.Error())
		}
		loop += 1
	}

	return containsString(okayResponses, responseStr)
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
			response = ask()
		} else {
			fmt.Print("\nWould you like to update it? ")
			response = ask()
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

func addToList(listA []string, listB []string) []string {
	sizeOfArray := len(listA) + len(listB)
	finalSlice := make([]string, sizeOfArray)
	counter := 0

	for index, _ := range listB {
		finalSlice[counter] = listB[index]
		counter += 1
	}

	for index, _ := range listA {
		finalSlice[counter] = listA[index]
		counter += 1
	}

	return finalSlice
}

func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}

func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

func updateRequirements(directory string, packages []PackageModel) {
	getVersion := func(pkg string, packs []PackageModel) string {
		for _, item := range packs {
			if pkg == item.Package {
				return item.NewVersion
			}
		}
		log.Panic("Package " + pkg + " not found")
		return ""
	}

	for versionFile := range findAllFilesToUpdate(directory) {
		fileData := strings.Split(string(versionFile[0]), "\n")
		var finalFile string

		for _, data := range fileData {
			if data == "" {
				continue
			}

			if strings.Contains(data, "==") {
				version := strings.Split(data, "==")
				newVersion := getVersion(version[0], packages)

				if newVersion != "" {
					finalFile += version[0] + "==" + newVersion + "\n"
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

			finalFile += data + "==" + newVersion + "\n"
		}
		updateRepo(versionFile[1], finalFile)
	}
}

func updateRepo(privateRequirementsPath string, finalFile string) {

	r, err := git.PlainOpen(strings.Replace(privateRequirementsPath, "private-requirements.txt", "", -1))
	if err != nil {
		log.Panic("Error while opening local git repository " + err.Error())
	}

	w, err := r.Worktree()
	if err != nil {
		log.Panic(err.Error())
	}

	out := strings.TrimRight(privateRequirementsPath, "/private-requirements.txt")
	currentPackage := out[strings.LastIndex(out, "/")+1:]

	for _, branch := range [2]string{"develop", "release"} {

		fmt.Print("\nDo you want to update the branch: " + branch + " of " + currentPackage + "? ")
		doUpdate := ask()

		if doUpdate {
			err = w.Reset(&git.ResetOptions{Mode: git.ResetMode(git.HardReset)})
			if err != nil {
				log.Panic(err.Error())
			}

			err = w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch)})
			if err != nil {
				log.Panic(err.Error())
			}

			err = w.Pull(&git.PullOptions{RemoteName: "origin", SingleBranch: true, ReferenceName: plumbing.ReferenceName(branch)})
			if err != nil {
				log.Panic(err.Error())
			}

			err := ioutil.WriteFile(privateRequirementsPath, []byte(finalFile), 0755)
			if err != nil {
				log.Panic("Error saving file " + err.Error())
			}

			_, err = w.Commit("Updating Packages via script...", &git.CommitOptions{Author: &object.Signature{Name: "Bruno Rodrigues", Email: "bruno.dasilva@ambevtech.com.br", When: time.Now()}})
			if err != nil {
				log.Panic(err.Error())
			}

			err = r.Push(&git.PushOptions{RemoteName: branch, RefSpecs: []config.RefSpec{"refs/remotes/origin/*"}})
			if err != nil {
				log.Panic(err.Error())
			}
		}
	}
}

func getPackagesRequirements(fatalErrors *chan string, directory string, packages *[]PackageModel) {
	for versionFile := range findAllFilesToUpdate(directory) {
		fileData := strings.Split(string(versionFile[0]), "\n")

		for _, data := range fileData {
			if data == "" {
				continue
			}

			if strings.Contains(data, "==") {
				version := strings.Split(data, "==")
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

func findAllFilesToUpdate(directory string) <-chan [2]string {
	response := make(chan [2]string)
	go func() {
		for _, match := range getAllProjects(directory) {
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

func getAllProjects(defaultDirectory string) []string {
	matches, err := filepath.Glob(defaultDirectory + "**/private-requirements.txt")
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

func validateArgs() {
	arguments := os.Args[1:]

	if len(arguments) == 0 {
		log.Fatalln("You should pass something")
	}

	if _, err := os.Stat(arguments[0]); os.IsNotExist(err) {
		log.Fatalln("The argument " + arguments[0] + " is not a valid folder")
	}
}

func main() {
	fmt.Println("Starting to update canaa-packages")
	initApplication()
	fmt.Println("Ending Process")
}
