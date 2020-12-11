package utils

import (
	"fmt"
	"log"
)

func Ask(question string) bool {
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
