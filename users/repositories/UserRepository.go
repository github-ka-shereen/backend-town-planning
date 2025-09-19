package repositories

import (
	"errors"
	"fmt"
	"strings"
	"town-planning-backend/db/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserRepository interface {
	CreateUser(user *models.User) (*models.User, error)
	GetUserByID(id string) (*models.User, error)
	GetUserByPhoneNumber(phone string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	UpdateUser(user *models.User) (*models.User, error)
	DeleteUser(id string) error
	GetAllUsers() ([]models.User, error)
	GetAllPermissions() ([]models.Permission, error)
	GetAllRoles() ([]models.Role, error)
	GetRoleWithPermissionsByID(roleID string) (*models.Role, error)
	CreateDepartment(department *models.Department) (*models.Department, error)
	GetDepartmentsAll() ([]models.Department, error)
	GetFilteredUsers(pageSize int, offset int, filters map[string]string) ([]models.User, int64, error)
}

// Implementations
type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetDepartmentsAll() ([]models.Department, error) {
	var departments []models.Department
	err := r.db.Find(&departments).Error
	return departments, err
}

func (r *userRepository) CreateDepartment(department *models.Department) (*models.Department, error) {
	// Check if department with same name already exists (including soft-deleted)
	var existing models.Department
	err := r.db.Unscoped().Where("name = ?", department.Name).First(&existing).Error
	if err == nil {
		// Department found
		if existing.DeletedAt.Valid {
			// Soft-deleted: restore and update
			existing.DeletedAt = gorm.DeletedAt{}
			existing.Description = department.Description
			existing.IsActive = department.IsActive
			existing.Email = department.Email
			existing.PhoneNumber = department.PhoneNumber
			existing.OfficeLocation = department.OfficeLocation
			existing.CreatedBy = department.CreatedBy

			if err := r.db.Unscoped().Save(&existing).Error; err != nil {
				return nil, fmt.Errorf("failed to restore soft-deleted department: %w", err)
			}
			return &existing, nil
		} else {
			// Active department with same name already exists
			return nil, fmt.Errorf("a department with that name already exists")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Unexpected DB error
		return nil, fmt.Errorf("failed to check for existing department: %w", err)
	}

	// Create new department
	if err := r.db.Create(department).Error; err != nil {
		return nil, fmt.Errorf("failed to create department in database: %w", err)
	}

	return department, nil
}

func (r *userRepository) GetRoleWithPermissionsByID(roleID string) (*models.Role, error) {
	var role models.Role
	err := r.db.Preload("Permissions.Permission").Where("id = ?", roleID).First(&role).Error
	return &role, err
}

func (r *userRepository) GetAllRoles() ([]models.Role, error) {
	var roles []models.Role
	err := r.db.Find(&roles).Error
	return roles, err
}

func (r *userRepository) GetAllPermissions() ([]models.Permission, error) {
	var permissions []models.Permission
	err := r.db.Find(&permissions).Error
	return permissions, err
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (r *userRepository) GetFilteredUsers(pageSize int, offset int, filters map[string]string) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	db := r.db.Model(&models.User{}) // start a new query chain

	// Apply filters
	for key, value := range filters {
		switch key {
		case "active":
			if strings.ToLower(value) == "true" {
				db = db.Where("is_active = ?", true)
			} else if strings.ToLower(value) == "false" {
				db = db.Where("is_active = ?", false)
			}
		case "start_date":
			db = db.Where("created_at >= ?", value)
		case "end_date":
			db = db.Where("created_at <= ?", value)
		}
	}

	// Count total records with filters applied
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if err := db.Limit(pageSize).Offset(offset).Order("created_at desc").Preload("Role").Preload("Department").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *userRepository) CreateUser(user *models.User) (*models.User, error) {
	// Hash password
	hashedPassword, err := HashPassword(user.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Check if user exists (including soft-deleted)
	var existing models.User
	err = r.db.Unscoped().Where("email = ?", user.Email).First(&existing).Error
	if err == nil {
		// Email found
		if existing.DeletedAt.Valid {
			// Soft-deleted: restore
			existing.DeletedAt = gorm.DeletedAt{}
			existing.FirstName = user.FirstName
			existing.LastName = user.LastName
			existing.Password = hashedPassword
			existing.Phone = user.Phone
			existing.Role = user.Role
			existing.Active = user.Active
			existing.CreatedBy = user.CreatedBy

			if err := r.db.Unscoped().Save(&existing).Error; err != nil {
				return nil, fmt.Errorf("failed to restore soft-deleted user: %w", err)
			}
			return &existing, nil
		} else {
			// Active user with same email already exists
			return nil, fmt.Errorf("a user with that email already exists")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Unexpected DB error
		return nil, fmt.Errorf("failed to check for existing user: %w", err)
	}

	// Create a new user
	user.ID = uuid.New()
	user.Password = hashedPassword

	if err := r.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user in database: %w", err)
	}

	return user, nil
}

func (r *userRepository) GetUserByID(id string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "id = ?", id).Error
	if user.Active == false || user.IsSuspended {
		return nil, fmt.Errorf("user account is disabled")
	}
	return &user, err
}

func (r *userRepository) GetUserByPhoneNumber(phone string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "phone = ?", phone).Error
	return &user, err
}

func (r *userRepository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "email = ?", email).Error
	return &user, err
}

func (r *userRepository) GetUsers() ([]models.User, error) {
	var users []models.User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *userRepository) UpdateUser(user *models.User) (*models.User, error) {
	result := r.db.Save(user)
	if result.Error != nil {
		return nil, result.Error
	}
	// After saving, the 'user' object should have the updated values.
	// We can directly return it. GORM updates the fields in the passed-in struct.
	return user, nil
}

func (r *userRepository) GetAllUsers() ([]models.User, error) {
	var users []models.User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *userRepository) DeleteUser(id string) error {
	return r.db.Delete(&models.User{}, "id = ?", id).Error
}
