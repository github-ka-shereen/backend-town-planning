package controllers

import (
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/stands/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (sc *StandController) CreateProject(c *fiber.Ctx) error {
	var project models.Project
	if err := c.BodyParser(&project); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid request",
			"error":   err.Error(),
		})
	}

	// Validate the project
	if validationError := services.ValidateProject(&project); validationError != "" {
		return c.Status(400).JSON(fiber.Map{
			"message": "Validation failed",
			"error":   validationError,
		})
	}

	// Check for duplicate project number
	existingProject, _ := sc.StandRepo.GetProjectByProjectNumber(project.ProjectNumber)

	if existingProject != nil {
		// Return an error response indicating the duplicate project number
		return c.Status(409).JSON(fiber.Map{
			"message":        "Duplicate project number",
			"error":          "A project with this project number already exists.",
			"project_number": project.ProjectNumber,
		})
	}

	// Save the new project using the repository
	createdProject, err := sc.StandRepo.CreateProject(&project)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Failed to create project",
			"error":   err.Error(),
		})
	}

	// Index the project in Elasticsearch
	if sc.BleveRepo != nil {
		err := sc.BleveRepo.IndexSingleProject(*createdProject)
		if err != nil {
			config.Logger.Error("Error indexing project", zap.Error(err), zap.String("projectID", createdProject.ID.String()))
		} else {
			config.Logger.Info("Successfully indexed project in Bleve", zap.String("projectID", createdProject.ID.String()), zap.Any("Project", createdProject))
		}
	} else {
		config.Logger.Warn("IndexingService is nil, skipping document indexing", zap.String("projectID", createdProject.ID.String()))
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Project created successfully",
		"data":    createdProject,
	})
}
