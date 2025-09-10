package controllers

import (
	"town-planning-backend/bleve/repositories"
)

type SearchController struct {
	repo *repositories.BleveRepository
}

func NewSearchController(repo *repositories.BleveRepository) *SearchController {
	return &SearchController{repo: repo}
}
