package budget

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// TaskBudget represents a budget for a specific task
type TaskBudget struct {
	Name        string
	Description string
	Amount      float64
}

// SetupTaskBudgets guides the user through setting up task budgets
func SetupTaskBudgets() (map[string]float64, error) {
	reader := bufio.NewReader(os.Stdin)
	budgets := make(map[string]float64)

	// Define the task types and their descriptions
	tasks := []TaskBudget{
		{
			Name:        "issue_response",
			Description: "Responding to GitHub issues",
			Amount:      0,
		},
		{
			Name:        "pr_creation",
			Description: "Creating pull requests",
			Amount:      0,
		},
		{
			Name:        "test_run",
			Description: "Running tests",
			Amount:      0,
		},
		{
			Name:        "default",
			Description: "Default budget for other tasks",
			Amount:      0,
		},
	}

	fmt.Println("Set maximum budget (in USD) for each task type:")
	fmt.Println("(This limits how much will be spent on each operation)")

	// Ask for budget for each task type
	for i, task := range tasks {
		fmt.Printf("\n%d. %s ($): ", i+1, task.Description)
		budgetStr, _ := reader.ReadString('\n')
		budgetStr = strings.TrimSpace(budgetStr)

		// Parse the budget amount
		amount, err := strconv.ParseFloat(budgetStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid budget amount: %v", err)
		}

		// Validate the amount
		if amount < 0 {
			return nil, fmt.Errorf("budget amount cannot be negative")
		}

		// Store the budget
		budgets[task.Name] = amount
	}

	// Show the configured budgets
	fmt.Println("\nConfigured budgets:")
	for _, task := range tasks {
		fmt.Printf("- %s: $%.2f\n", task.Description, budgets[task.Name])
	}

	// Confirm the budgets
	fmt.Println("\nAre these budgets correct? (y/n)")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		return nil, fmt.Errorf("budget configuration canceled")
	}

	return budgets, nil
}
