package employeerds

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RDSHandle struct {
	AWSConfig           aws.Config
	PostgresConn        *pgxpool.Pool
	TableName           string
	SecretManagerString string
	CreateEmployeeQuery string
	GetEmployeeQuery    string
	RemoveEmployeeQuery string
	UpdateEmployeeQuery string
}

type Employee struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Field     string    `json:"field"`
	StartTime string    `json:"start_time"`
	DOB       string    `json:"dob"`
	Salary    int       `json:"salary"`
}

type Secret struct {
	Host     string
	Port     int
	Username string
	Password string
	DBName   string
}

func NewRDSHandleMust() *RDSHandle {
	h := &RDSHandle{}

	h.SecretManagerString = os.Getenv("RDS_SECRET")
	h.TableName = os.Getenv("RDS_TABLE_NAME")

	if h.SecretManagerString == "" {
		log.Fatal("rds environment variables not set")
	}
	var err error
	h.AWSConfig, err = config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Failed to initialize the config: %v", err)
	}

	svc := secretsmanager.NewFromConfig(h.AWSConfig)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(h.SecretManagerString),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}
	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		log.Fatalf("Failed to get the secret value: %v", err.Error())
	}

	var secret Secret
	err = json.Unmarshal([]byte(*result.SecretString), &secret)
	if err != nil {
		log.Fatalf("Failed to parse secret value: %v", err)
	}

	// Construct the connection string
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d",
		secret.Host, secret.Username, secret.Password, secret.DBName, secret.Port)

	h.PostgresConn, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Adjust the timeout value as needed
	defer cancel()

	if err = h.PostgresConn.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping the database: %v", err)
	}

	h.CreateEmployeeQuery = fmt.Sprintf(`INSERT INTO %s (id, first_name, last_name, field, start_time, dob, salary) VALUES ($1, $2, $3, $4, $5, $6, $7);`, h.TableName)
	h.GetEmployeeQuery = fmt.Sprintf(`SELECT id, first_name, last_name, field, start_time, dob, salary FROM %s WHERE id = $1;`, h.TableName)
	h.RemoveEmployeeQuery = fmt.Sprintf(`DELETE FROM %s WHERE id = $1;`, h.TableName)
	h.UpdateEmployeeQuery = `UPDATE %s SET %s WHERE id = '%s';`

	return h
}

func (h *RDSHandle) CreateEmployee(c *fiber.Ctx) error {

	customContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	employee := &Employee{}
	if err := c.BodyParser(employee); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
			"status":  "fail",
			"message": err.Error(),
		})
	}

	_, err := h.PostgresConn.Exec(customContext, h.CreateEmployeeQuery, employee.ID, employee.FirstName, employee.LastName, employee.Field, employee.StartTime, employee.DOB, employee.Salary)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"status":  "fail",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(&fiber.Map{
		"status":  "success",
		"message": "Employee created successfully",
	})
}

func (h *RDSHandle) GetEmployee(c *fiber.Ctx) error {

	customContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	type EmployeeFix struct { // date must be a string for input, and must be a time.Time for output.
		ID        uuid.UUID `json:"id"`
		FirstName string    `json:"first_name"`
		LastName  string    `json:"last_name"`
		Field     string    `json:"field"`
		StartTime time.Time `json:"start_time"`
		DOB       time.Time `json:"dob"`
		Salary    int       `json:"salary"`
	}

	employee := &EmployeeFix{}

	employeeID := c.Params("id")
	if employeeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
			"status":  "fail",
			"message": "Employee ID is required",
		})
	}

	err := h.PostgresConn.QueryRow(customContext, h.GetEmployeeQuery, employeeID).Scan(
		&employee.ID,
		&employee.FirstName,
		&employee.LastName,
		&employee.Field,
		&employee.StartTime,
		&employee.DOB,
		&employee.Salary,
	)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"status":  "fail",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(employee)
}

func (h *RDSHandle) RemoveEmployee(c *fiber.Ctx) error {

	customContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	employeeID := c.Params("id")
	if employeeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
			"status":  "fail",
			"message": "Employee ID is required",
		})
	}

	_, err := h.PostgresConn.Exec(customContext, h.RemoveEmployeeQuery, employeeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"status":  "fail",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(&fiber.Map{
		"status":  "success",
		"message": "Employee successfully deleted",
	})
}

func HasField(key string, s interface{}) bool {
	rt := reflect.TypeOf(s)
	if rt.Kind() != reflect.Struct {
		return false
	}

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		v := strings.Split(f.Tag.Get("json"), ",")[0]
		if v == "" || v == "-" {
			continue
		}
		if v == key {
			return true
		}
	}
	return false
}

func (h *RDSHandle) UpdateEmployee(c *fiber.Ctx) error {

	customContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	fields := map[string]any{}
	if err := c.BodyParser(&fields); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
			"status":  "fail",
			"message": err.Error(),
		})
	}

	employeeID := c.Params("id")
	if employeeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
			"status":  "fail",
			"message": "Employee ID is required",
		})
	}

	updateSQLStatement := ""
	var err error

	for key, value := range fields {

		if !HasField(key, Employee{}) {
			return c.Status(fiber.StatusBadRequest).JSON(&fiber.Map{
				"status":  "fail",
				"message": fmt.Sprintf("Failed to update. Key: %s is not a valid name", key),
			})
		}

		updateSQLStatement += fmt.Sprintf("%s = '%v', ", key, value)
	}

	// remove the last comma
	updateSQLStatement = updateSQLStatement[:len(updateSQLStatement)-2]

	sqlStatement := fmt.Sprintf(h.UpdateEmployeeQuery, h.TableName, updateSQLStatement, employeeID)

	_, err = h.PostgresConn.Exec(customContext, sqlStatement)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(&fiber.Map{
			"status":  "fail",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(&fiber.Map{
		"status":  "success",
		"message": "Employee successfully updated",
	})

}
