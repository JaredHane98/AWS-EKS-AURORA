package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"math/rand"

	"github.com/google/uuid"
)

type Employee struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Sector    string    `json:"sector"`
	StartTime string    `json:"start_time"`
	BirthDate string    `json:"dob"`
	Salary    int       `json:"salary"`
}

type EmployeeList struct {
	Employees []Employee `json:"employees"`
}

var Sectors = []string{"Enginering", "Sales", "Marketing", "HR", "Finance", "Management", "IT"}

func ReadNames(fileName string) ([]string, error) {

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lineScanner := bufio.NewScanner(file)

	var names []string
	for lineScanner.Scan() {
		names = append(names, lineScanner.Text())
	}
	if lineScanner.Err() != nil {
		return nil, lineScanner.Err()
	}
	return names, nil
}

func getRandomDate(startYear, startMonth, startDay, endYear, endMonth, endDay int) string {

	min := time.Date(startYear, time.Month(startMonth), startDay, 0, 0, 0, 0, time.UTC).Unix()
	max := time.Date(endYear, time.Month(endMonth), endDay, 0, 0, 0, 0, time.UTC).Unix()

	delta := max - min

	sec := rand.Int63n(delta) + min
	cur := time.Unix(sec, 0)
	return fmt.Sprintf("%v-%v-%v", cur.Year(), int(cur.Month()), cur.Day())
}

func GetRandomSalary(base, salaryRange int) int {
	return rand.Intn(salaryRange) + base
}

func GetRandomID() string {
	return uuid.New().String()
}

func GetRandomSector() string {
	return Sectors[rand.Intn(len(Sectors))]
}

func main() {
	firstNames, err := ReadNames("first-names.txt")
	if err != nil || len(firstNames) == 0 {
		log.Fatalf("Failed to read the first names: %v", err)
	}
	lastNames, err := ReadNames("last-names.txt")
	if err != nil || len(lastNames) == 0 {
		log.Fatalf("Failed to read the last names: %v", err)
	}

	numNames := min(len(firstNames), len(lastNames))

	var employeeList EmployeeList
	employeeList.Employees = make([]Employee, 0, numNames)

	for i := 0; i < numNames; i++ {
		employee := Employee{
			ID:        uuid.New(),
			FirstName: firstNames[rand.Intn(numNames)],
			LastName:  lastNames[rand.Intn(numNames)],
			Sector:    GetRandomSector(),
			StartTime: getRandomDate(2010, 1, 1, 2024, 12, 31),
			BirthDate: getRandomDate(1960, 1, 1, 2000, 12, 31),
			Salary:    GetRandomSalary(50000, 100000),
		}

		employeeList.Employees[i] = employee
	}

	bytes, err := json.MarshalIndent(employeeList, "", "\t")
	if err != nil {
		log.Fatalf("Failed to marshal the employee list: %v", err)
	}

	outputFile, err := os.OpenFile("output-database.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatalf("Failed to create output file file: %v", err)
	}

	if _, err = outputFile.Write(bytes); err != nil {
		log.Fatalf("Failed to write to file: %v", err)
	}

	if err = outputFile.Close(); err != nil {
		log.Fatalf("Failed to close file: %v", err)
	}
}
