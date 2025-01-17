package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"project/internal/data"
	"project/utils"
	"project/utils/validator"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (app *application) CreatePreProjectHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	usersID, err := uuid.Parse(userID)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user, err := app.Model.UserDB.GetUser(usersID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}

	// existingPreProject, err := app.Model.PreProjectDB.CheckExistingPreProject(user.ID)
	// if err != nil {
	// 	app.serverErrorResponse(w, r, err)
	// 	return
	// }
	// if existingPreProject != nil {
	// 	app.errorResponse(w, r, http.StatusConflict, "You already have an existing pre-project")
	// 	return
	// }

	name := r.FormValue("name")
	description := r.FormValue("description")
	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil {
		app.errorResponse(w, r, http.StatusBadRequest, "Invalid year")
		return
	}
	season := r.FormValue("season")

	var file *string
	if uploadedFile, fileHeader, err := r.FormFile("file"); err == nil {
		defer uploadedFile.Close()
		fileName, err := utils.SaveFile(uploadedFile, "pre_projects", fileHeader.Filename)
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid file")
			return
		}
		file = &fileName
	} else if err != http.ErrMissingFile {
		app.errorResponse(w, r, http.StatusBadRequest, "Invalid file upload")
		return
	}

	advisorsEmails := strings.Split(r.FormValue("advisors"), ",")
	var advisorIDs []uuid.UUID
	for _, email := range advisorsEmails {
		advisor, err := app.Model.UserDB.GetUserByEmail(email)
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid advisor email")
			return
		}
		advisorIDs = append(advisorIDs, advisor.ID)
	}

	var studentIDs []uuid.UUID
	studentIDs = append(studentIDs, user.ID)

	addedStudentIDs := map[uuid.UUID]bool{
		user.ID: true,
	}
	if r.FormValue("students") != "" {
		studentsEmails := strings.Split(r.FormValue("students"), ",")
		for _, email := range studentsEmails {
			student, err := app.Model.UserDB.GetUserByEmail(email)
			if err != nil {
				app.errorResponse(w, r, http.StatusBadRequest, "Invalid student email")
				return
			}

			existingStudentPreProject, err := app.Model.PreProjectDB.CheckExistingPreProject(student.ID)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}
			if existingStudentPreProject != nil {
				app.errorResponse(w, r, http.StatusConflict, fmt.Sprintf("Student with email %s already has an existing pre-project", email))
				return
			}

			if !addedStudentIDs[student.ID] {
				studentIDs = append(studentIDs, student.ID)
				addedStudentIDs[student.ID] = true
			}
		}
	}
	fileDescription := r.FormValue("file_description")

	preProject := data.PreProject{
		ID:              uuid.New(),
		Name:            name,
		Description:     &description,
		File:            file,
		FileDescription: &fileDescription,
		Year:            year,
		Season:          season,
		ProjectOwner:    usersID,
		CanUpdate:       true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	v := validator.New()
	data.ValidatePreProject(v, &preProject, studentIDs, advisorIDs)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// similarityCheckResp, err := utils.CheckProjectSimilarity(name, description, 0.3)
	// if err != nil {
	// 	app.serverErrorResponse(w, r, err)
	// 	return
	// }

	// var highSimilarityProjects []map[string]interface{}

	// for _, project := range similarityCheckResp.SimilarProjects {
	// 	similarityScore, ok := project["similarity_score"].(float64)
	// 	sourceTable, tableOk := project["source_table"].(string)
	// 	if !ok || !tableOk {
	// 		continue
	// 	}

	// 	if similarityScore > 50 {
	// 		similarProject := map[string]interface{}{
	// 			"project_id":          project["project_id"],
	// 			"project_name":        project["project_name"],
	// 			"project_description": project["project_description"],
	// 			"similarity_score":    similarityScore,
	// 			"source_table":        sourceTable,
	// 		}
	// 		highSimilarityProjects = append(highSimilarityProjects, similarProject)
	// 	}
	// }

	// if len(highSimilarityProjects) > 0 {
	// 	response := utils.Envelope{
	// 		"error":            "Similar projects found",
	// 		"similar_projects": highSimilarityProjects,
	// 		"message":          "Project is too similar to existing projects. Please modify your project.",
	// 	}
	// 	app.errorResponse(w, r, http.StatusConflict, response)
	// 	return
	// }

	err = app.Model.PreProjectDB.InsertPreProject(&preProject, studentIDs, advisorIDs)
	if err != nil {
		if preProject.File != nil {
			utils.DeleteFile(*preProject.File)
		}
		app.serverErrorResponse(w, r, err)
		return
	}
	utils.SendJSONResponse(w, http.StatusCreated, utils.Envelope{"pre_project": preProject})
}
func (app *application) GetPreProjectsHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	id := queryParams.Get("id")
	if id != "" {
		// Dynamically add the ID to the filters
		filters := queryParams.Get("filters")
		if filters == "" {
			queryParams.Set("filters", fmt.Sprintf("id:%s", id))
		} else {
			queryParams.Set("filters", fmt.Sprintf("%s,id:%s", filters, id))
		}
	}

	preProjects, meta, err := app.Model.PreProjectDB.ListPreProjects(queryParams)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, utils.Envelope{"pre_projects": preProjects, "meta": meta})
}

func (app *application) GetPreProjectsHandlerByID(w http.ResponseWriter, r *http.Request) {
	preProjectID := uuid.MustParse(r.PathValue("id"))

	preProject, err := app.Model.PreProjectDB.GetPreProjectWithAdvisorDetails(preProjectID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, utils.Envelope{
		"pre_project": preProject,
	})
}
func (app *application) UpdatePreProjectHandler(w http.ResponseWriter, r *http.Request) {
	userRoles, ok := r.Context().Value(UserRoleKey).([]string)
	if !ok {
		app.unauthorizedResponse(w, r)
		return
	}
	isAdmin := false
	for _, role := range userRoles {
		if role == "admin" {
			isAdmin = true
			break
		}
	}

	preProjectID := uuid.MustParse(r.PathValue("id"))

	existingPreProject, err := app.Model.PreProjectDB.GetPreProjectWithAdvisorDetails(preProjectID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}

	preProject := &data.PreProject{
		ID:           preProjectID,
		ProjectOwner: existingPreProject.PreProject.ProjectOwner,
		UpdatedAt:    time.Now(),
	}
	// nameChanged := false
	// descriptionChanged := false

	name := r.FormValue("name")
	if name != "" {
		preProject.Name = name
		// nameChanged = true
	} else {
		preProject.Name = existingPreProject.PreProject.Name
	}

	canUpdateStr := r.FormValue("can_update")
	var canUpdate bool
	if canUpdateStr != "" {
		var err error
		canUpdate, err = strconv.ParseBool(canUpdateStr)
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid can_update value")
			return
		}
		preProject.CanUpdate = canUpdate
	} else {
		preProject.CanUpdate = existingPreProject.PreProject.CanUpdate
	}
	description := r.FormValue("description")
	if description != "" {
		preProject.Description = &description
		// descriptionChanged = true
	} else {
		preProject.Description = existingPreProject.PreProject.Description
	}

	yearStr := r.FormValue("year")
	var year int
	if yearStr != "" {
		year, err = strconv.Atoi(yearStr)
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid year")
			return
		}
		preProject.Year = year
	} else {
		preProject.Year = existingPreProject.PreProject.Year
	}

	season := r.FormValue("season")
	if season != "" {
		preProject.Season = season
	} else {
		preProject.Season = existingPreProject.PreProject.Season
	}

	degree := r.FormValue("degree")
	if degree != "" && isAdmin {
		intDegree, err := strconv.Atoi(degree)
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid degree")
			return
		}
		preProject.Degree = &intDegree
	} else {
		preProject.Degree = existingPreProject.PreProject.Degree
	}
	var file *string
	var oldFile *string
	if uploadedFile, fileHeader, err := r.FormFile("file"); err == nil {
		defer uploadedFile.Close()
		fileName, err := utils.SaveFile(uploadedFile, "pre_projects", fileHeader.Filename)
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid file")
			return
		}
		file = &fileName
		oldFile = existingPreProject.PreProject.File
		preProject.File = file
	} else if err != http.ErrMissingFile {
		app.errorResponse(w, r, http.StatusBadRequest, "Invalid file upload")
		return
	} else {
		if existingPreProject.PreProject.File != nil {
			*existingPreProject.PreProject.File = strings.TrimPrefix(*existingPreProject.PreProject.File, data.Domain+"/")
		}

		preProject.File = existingPreProject.PreProject.File
	}
	fileDescription := r.FormValue("file_description")
	if fileDescription != "" {
		preProject.FileDescription = &fileDescription
	} else if existingPreProject.PreProject.FileDescription != nil {
		preProject.FileDescription = existingPreProject.PreProject.FileDescription
	}
	// Determine if user lists are being updated
	studentsProvided := r.FormValue("students") != ""
	advisorsProvided := r.FormValue("advisors") != ""

	// Parse students
	var students []uuid.UUID
	if studentsProvided {
		studentEmails := r.FormValue("students")
		studentEmailList := strings.Split(studentEmails, ",")
		for _, email := range studentEmailList {
			email = strings.TrimSpace(email)
			if email == "" {
				continue
			}
			student, err := app.Model.UserDB.GetUserByEmail(email)
			if err != nil {
				app.errorResponse(w, r, http.StatusBadRequest, "Invalid student email")
				return
			}
			students = append(students, student.ID)
		}
		// Ensure at least one student is provided when updating
		if len(students) == 0 {
			app.badRequestResponse(w, r, errors.New("at least one student is required"))
			return
		}
	} else {
		// Use existing students if no new students are provided
		for _, student := range existingPreProject.Students {
			students = append(students, student.StudentID)
		}
	}

	// Parse advisors
	var advisors []uuid.UUID
	if advisorsProvided {
		advisorEmails := r.FormValue("advisors")
		advisorEmailList := strings.Split(advisorEmails, ",")
		for _, email := range advisorEmailList {
			email = strings.TrimSpace(email)
			if email == "" {
				continue
			}
			advisor, err := app.Model.UserDB.GetUserByEmail(email)
			if err != nil {
				app.errorResponse(w, r, http.StatusBadRequest, "Invalid advisor email")
				return
			}
			advisors = append(advisors, advisor.ID)
		}
		// Ensure at least one advisor is provided when updating
		if len(advisors) == 0 {
			app.badRequestResponse(w, r, errors.New("at least one advisor is required"))
			return
		}
	} else {
		// Use existing advisors if no new advisors are provided
		for _, advisor := range existingPreProject.Advisors {
			advisors = append(advisors, advisor.AdvisorID)
		}
	}
	discutantsProvided := r.FormValue("discutants") != ""

	// Parse discussants
	var discutants []uuid.UUID
	if discutantsProvided {
		discussantEmails := r.FormValue("discutants")
		discussantEmailList := strings.Split(discussantEmails, ",")
		for _, email := range discussantEmailList {
			email = strings.TrimSpace(email)
			if email == "" {
				continue
			}
			discussant, err := app.Model.UserDB.GetUserByEmail(email)
			if err != nil {
				app.errorResponse(w, r, http.StatusBadRequest, "Invalid discussant email")
				return
			}
			discutants = append(discutants, discussant.ID)
		}
	} else {
		// Use existing discussants if no new discussants are provided
		for _, discussant := range existingPreProject.Discussants {
			discutants = append(discutants, discussant.DiscussantID)
		}
	}

	if !discutantsProvided {
		err := app.Model.PreProjectDB.RemoveAllDiscussants(preProject.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		// Avoid re-adding the removed discussants
		discutants = []uuid.UUID{}
	}

	v := validator.New()
	data.ValidatePreProject(v, preProject, students, advisors)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// if nameChanged || descriptionChanged {
	// 	similarityCheckResp, err := utils.CheckProjectSimilarity(preProject.Name, *preProject.Description, 0.3)
	// 	if err != nil {
	// 		app.serverErrorResponse(w, r, err)
	// 		return
	// 	}

	// 	var highSimilarityProjects []map[string]interface{}

	// 	for _, project := range similarityCheckResp.SimilarProjects {
	// 		similarityScore, ok := project["similarity_score"].(float64)
	// 		sourceTable, tableOk := project["source_table"].(string)
	// 		if !ok || !tableOk {
	// 			continue
	// 		}

	// 		if similarityScore > 50 {
	// 			similarProject := map[string]interface{}{
	// 				"project_id":          project["project_id"],
	// 				"project_name":        project["project_name"],
	// 				"project_description": project["project_description"],
	// 				"similarity_score":    similarityScore,
	// 				"source_table":        sourceTable,
	// 			}
	// 			highSimilarityProjects = append(highSimilarityProjects, similarProject)
	// 		}
	// 	}

	// 	if len(highSimilarityProjects) > 0 {
	// 		response := utils.Envelope{
	// 			"error":            "Similar projects found",
	// 			"similar_projects": highSimilarityProjects,
	// 			"message":          "Project is too similar to existing projects. Please modify your project.",
	// 		}
	// 		app.errorResponse(w, r, http.StatusConflict, response)
	// 		return
	// 	}
	// }

	err = app.Model.PreProjectDB.UpdatePreProject(preProject, advisors, students, discutants)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if oldFile != nil {
		*oldFile = strings.TrimPrefix(*oldFile, data.Domain+"/")

		if err := utils.DeleteFile(*oldFile); err != nil {
			log.Printf("Failed to delete old file %s: %v", *oldFile, err)
		}
	}

	updatedPreProject, err := app.Model.PreProjectDB.GetPreProjectWithAdvisorDetails(preProjectID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, utils.Envelope{
		"pre_project": updatedPreProject,
	})
}

func (app *application) DeletePreProjectHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	studentUUID, err := uuid.Parse(userID)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	preProjectID := uuid.MustParse(r.PathValue("id"))

	err = app.Model.PreProjectDB.DeletePreProject(preProjectID, studentUUID)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "pre-project not found"):
			app.errorResponse(w, r, http.StatusNotFound, "Pre-project not found")
		case strings.Contains(err.Error(), "unauthorized to delete"):
			app.errorResponse(w, r, http.StatusForbidden, "Unauthorized to delete this pre-project")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, utils.Envelope{
		"message": "Pre-project deleted successfully",
	})
}
func (app *application) RespondToPreProjectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Context() == nil {
		app.errorResponse(w, r, http.StatusInternalServerError, "Invalid context")
		return
	}

	advisorIDValue := r.Context().Value(UserIDKey)
	if advisorIDValue == nil {
		app.errorResponse(w, r, http.StatusUnauthorized, "User ID not found in context")
		return
	}
	advisorID, ok := advisorIDValue.(string)
	if !ok {
		app.errorResponse(w, r, http.StatusInternalServerError, "Invalid user ID type")
		return
	}

	advisorUUID, err := uuid.Parse(advisorID)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("invalid advisor ID: %w", err))
		return
	}
	preProjectIDStr := r.FormValue("pre_project_id")
	status := r.FormValue("status")

	if preProjectIDStr == "" {
		app.errorResponse(w, r, http.StatusBadRequest, "Pre-project ID is required")
		return
	}
	if status == "" {
		app.errorResponse(w, r, http.StatusBadRequest, "Status is required")
		return
	}
	preProjectUUID, err := uuid.Parse(preProjectIDStr)
	if err != nil {
		app.errorResponse(w, r, http.StatusBadRequest, "Invalid pre-project ID")
		return
	}

	preProject, err := app.Model.PreProjectDB.GetPreProjectWithAdvisorDetails(preProjectUUID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}

	if preProject.PreProject.AcceptedAdvisor != nil && *preProject.PreProject.AcceptedAdvisor == advisorUUID {
		app.errorResponse(w, r, http.StatusConflict, "You Already Accepeted the Project!")
		return
	}

	if preProject == nil {
		app.errorResponse(w, r, http.StatusNotFound, "Pre-project not found")
		return
	}
	v := validator.New()
	advisorIDs := make([]uuid.UUID, len(preProject.Advisors))
	for i, advisor := range preProject.Advisors {
		advisorIDs[i] = advisor.AdvisorID
	}

	data.ValidateAdvisorResponse(v, advisorUUID, status, advisorIDs)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}
	err = app.Model.PreProjectDB.InsertAdvisorResponse(preProjectUUID, advisorUUID, status)
	if err != nil {
		switch {

		case strings.Contains(err.Error(), "already been accepted by another advisor"):
			app.errorResponse(w, r, http.StatusConflict, "Pre-project has already been accepted by another advisor")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	message := "Advisor response recorded successfully"
	if status == "accepted" {
		message = "Pre-project accepted successfully"
	}

	utils.SendJSONResponse(w, http.StatusOK, utils.Envelope{
		"message": message,
	})
}

func (app *application) ResetPreProjectAdvisorsHandler(w http.ResponseWriter, r *http.Request) {

	preProjectID := uuid.MustParse(r.PathValue("id"))
	err := app.Model.PreProjectDB.ResetPreProjectAdvisors(preProjectID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	updatedPreProject, err := app.Model.PreProjectDB.GetPreProjectWithAdvisorDetails(preProjectID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, utils.Envelope{
		"pre_project": updatedPreProject,
		"message":     "Pre-project advisors reset successfully",
	})
}
func (app *application) MovePreProjectToBookHandler(w http.ResponseWriter, r *http.Request) {
	preProjectID := uuid.MustParse(r.PathValue("id"))
	preProject, err := app.Model.PreProjectDB.GetPreProjectWithAdvisorDetails(preProjectID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}
	cleanedFilePath := ""
	if preProject.PreProject.File != nil {
		cleanedFilePath = strings.TrimPrefix(*preProject.PreProject.File, data.Domain+"/")

	}

	book := &data.Book{
		ID:          uuid.New(),
		Name:        preProject.PreProject.Name,
		Description: preProject.PreProject.Description,
		File:        &cleanedFilePath,
		Year:        preProject.PreProject.Year,
		Season:      preProject.PreProject.Season,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	studentIDs := make([]uuid.UUID, len(preProject.Students))
	for i, student := range preProject.Students {
		studentIDs[i] = student.StudentID
	}

	Discussants := make([]uuid.UUID, len(preProject.Discussants))
	for i, dis := range preProject.Discussants {
		Discussants[i] = dis.DiscussantID
	}
	advisorIDs := []uuid.UUID{*preProject.PreProject.AcceptedAdvisor}

	v := validator.New()
	data.ValidateBook(v, book, studentIDs, advisorIDs, Discussants, false)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.Model.BookDB.InsertBook(book, Discussants, advisorIDs, studentIDs)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.Model.PreProjectDB.DeletePreProject(preProjectID, preProject.PreProject.ProjectOwner)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	createdBook, err := app.Model.BookDB.GetBookWithDetails(book.ID)
	if err != nil {
		app.handleRetrievalError(w, r, err)
		return
	}

	utils.SendJSONResponse(w, http.StatusCreated, utils.Envelope{
		"book":    createdBook,
		"message": "Pre-project successfully moved to book",
	})
}
