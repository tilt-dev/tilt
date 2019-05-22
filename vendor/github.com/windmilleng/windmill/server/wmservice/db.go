package wmservice

import (
	"fmt"
	"os"
	"strings"
)

func NormalizeDB(dbType, dbAddr string) (string, string, error) {
	if dbType == "" && dbAddr != "" {
		return "", "", fmt.Errorf("db_addr specified but no db_type. Did you forget to pass the db_type flag?")
	} else if dbAddr == "" && dbType != "" {
		return "", "", fmt.Errorf("db_type specified but no db_addr. Did you forget to pass the db_addr flag?")
	}

	dbUser := os.Getenv("DB_USER")
	if strings.Contains(dbAddr, "{DB_USER}") {
		if dbUser == "" {
			return "", "", fmt.Errorf("Missing DB_USER env variable")
		}
		dbAddr = strings.Replace(dbAddr, "{DB_USER}", dbUser, -1)
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if strings.Contains(dbAddr, "{DB_PASSWORD}") {
		if dbPassword == "" {
			return "", "", fmt.Errorf("Missing DB_PASSWORD env variable")
		}
		dbAddr = strings.Replace(dbAddr, "{DB_PASSWORD}", dbPassword, -1)
	}

	return dbType, dbAddr, nil
}
