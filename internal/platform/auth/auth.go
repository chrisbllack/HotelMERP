package auth

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "net/http"
    "time"
    
    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"
)

// User represents a system user
type User struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Email    string `json:"email"`
    Role     string `json:"role"`
    Password string `json:"-"`
}

// Role defines user permissions
type Role struct {
    Name        string
    Permissions []string
}

// All roles with their permissions
var Roles = map[string]Role{
    "admin": {
        Name: "Administrator",
        Permissions: []string{
            "read_all", "write_all", "delete_all", "manage_users", "config_system",
        },
    },
    "manager": {
        Name: "Manager",
        Permissions: []string{
            "read_all", "write_all", "manage_reservations", "view_reports",
        },
    },
    "frontdesk": {
        Name: "Front Desk Staff",
        Permissions: []string{
            "read_guests", "write_guests", "read_reservations", "write_reservations",
            "check_in", "check_out", "assign_room",
        },
    },
    "housekeeping": {
        Name: "Housekeeping Staff",
        Permissions: []string{
            "read_rooms", "update_room_status", "view_housekeeping_tasks",
        },
    },
    "guest": {
        Name: "Guest",
        Permissions: []string{
            "view_profile", "view_reservations", "update_profile",
        },
    },
}

var (
    jwtSecret = []byte("your-secret-key-here")
    tokenExp  = time.Hour * 24 // 24 hours
)

// GenerateJWT creates a new JWT token for the user
func GenerateJWT(user User) (string, error) {
    claims := jwt.MapClaims{
        "user_id":   user.ID,
        "user_name": user.Name,
        "role":      user.Role,
        "exp":       time.Now().Add(tokenExp).Unix(),
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(jwtSecret)
}

// VerifyToken validates a JWT token
func VerifyToken(tokenString string) (*jwt.Token, error) {
    return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        return jwtSecret, nil
    })
}

// JWTMiddleware checks for valid JWT tokens
func JWTMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenString := r.Header.Get("Authorization")
        if tokenString == "" {
            http.Error(w, "Missing token", http.StatusUnauthorized)
            return
        }
        
        token, err := VerifyToken(tokenString)
        if err != nil || !token.Valid {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        // Add user info to context
        claims, ok := token.Claims.(jwt.MapClaims)
        if !ok {
            http.Error(w, "Invalid token claims", http.StatusUnauthorized)
            return
        }
        
        // Continue with the request
        ctx := context.WithValue(r.Context(), "user_id", claims["user_id"])
        ctx = context.WithValue(ctx, "user_role", claims["role"])
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Init initializes authentication system
func Init() {
    // Initialize any authentication setup
    // This would load roles from database or config
}
