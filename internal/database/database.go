package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

type Domain struct {
	ID           int64
	Domain       string
	Program      string
	Status       string // "up", "down", "unknown"
	DiscoveredAt time.Time
	LastChecked  time.Time
	IsNew        bool
}

type Program struct {
	ID             int64
	Name           string
	Handle         string
	URL            string
	Domain         string
	OffersBounties bool
	ProgramType    string // "RDP", "VDP", "BOTH", "UNKNOWN"
	LastScanned    time.Time
}

type StatusChange struct {
	ID          int64
	Domain      string
	Program     string
	OldStatus   string
	NewStatus   string
	ChangedAt   time.Time
	Notified    bool
}

type DomainInfo struct {
	Domain      string
	Program     string
	Status      string
	Title       string
	StatusCode  int
	Technologies []string
	LastChecked time.Time
}

func Init(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Migrate existing tables FIRST (before createTables)
	// This adds new columns to existing tables
	log.Println("Running database migrations...")
	if err := migrateTables(db); err != nil {
		log.Printf("Warning: Migration had errors (this may be OK): %v", err)
		// Don't fail - migration errors are often expected (columns already exist)
	}
	log.Println("Database migrations completed")

	// Then create tables (will skip if they exist, but will create with new schema if they don't)
	log.Println("Creating database tables...")
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &DB{db}, nil
}

func migrateTables(db *sql.DB) error {
	// Check if programs table exists first
	var tableExists int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='programs'`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		// Table doesn't exist yet, will be created by createTables with new schema
		return nil
	}

	// Try to add new columns to programs table
	// SQLite will error if column already exists, which we'll ignore
	migrations := []struct {
		table      string
		column     string
		definition string
	}{
		{"programs", "domain", "TEXT"},
		{"programs", "offers_bounties", "BOOLEAN DEFAULT 0"},
		{"programs", "program_type", "TEXT DEFAULT 'UNKNOWN'"},
	}

	for _, mig := range migrations {
		// Just try to add the column - SQLite will error if it exists, which is fine
		// Note: Table and column names are from our code, safe to use in fmt.Sprintf
		query := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, mig.table, mig.column, mig.definition)
		_, err := db.Exec(query)
		if err != nil {
			// Check if error is because column already exists
			errStr := err.Error()
			if strings.Contains(errStr, "duplicate column") || 
			   strings.Contains(errStr, "already exists") ||
			   strings.Contains(errStr, "duplicate column name") {
				// Column already exists, that's fine
				log.Printf("Column %s.%s already exists, skipping", mig.table, mig.column)
				continue
			}
			// Log other errors but don't fail
			log.Printf("Migration warning for %s.%s: %v", mig.table, mig.column, err)
		} else {
			log.Printf("Migrated: Added column %s.%s", mig.table, mig.column)
		}
	}
	return nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS programs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			handle TEXT UNIQUE NOT NULL,
			url TEXT,
			domain TEXT,
			offers_bounties BOOLEAN DEFAULT 0,
			program_type TEXT DEFAULT 'UNKNOWN',
			last_scanned DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS status_changes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT NOT NULL,
			program TEXT NOT NULL,
			old_status TEXT NOT NULL,
			new_status TEXT NOT NULL,
			changed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			notified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS domain_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT UNIQUE NOT NULL,
			program TEXT NOT NULL,
			status TEXT DEFAULT 'unknown',
			title TEXT,
			status_code INTEGER,
			technologies TEXT,
			last_checked DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS domains (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT NOT NULL,
			program TEXT NOT NULL,
			status TEXT DEFAULT 'unknown',
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_checked DATETIME,
			is_new BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(domain, program)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_program ON domains(program)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_status ON domains(status)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_is_new ON domains(is_new)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_discovered_at ON domains(discovered_at)`,
		`CREATE INDEX IF NOT EXISTS idx_status_changes_domain ON status_changes(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_status_changes_notified ON status_changes(notified)`,
		`CREATE INDEX IF NOT EXISTS idx_programs_type ON programs(program_type)`,
		`CREATE INDEX IF NOT EXISTS idx_programs_bounties ON programs(offers_bounties)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) SaveProgram(program *Program) error {
	// Try new schema first
	query := `INSERT OR REPLACE INTO programs (handle, name, url, domain, offers_bounties, program_type, last_scanned) 
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, program.Handle, program.Name, program.URL, program.Domain, 
		program.OffersBounties, program.ProgramType, time.Now())
	
	// If that fails due to missing columns, try old schema
	if err != nil && strings.Contains(err.Error(), "no such column") {
		query = `INSERT OR REPLACE INTO programs (handle, name, url, last_scanned) 
		         VALUES (?, ?, ?, ?)`
		_, err = db.Exec(query, program.Handle, program.Name, program.URL, time.Now())
	}
	
	return err
}

func (db *DB) GetPrograms() ([]Program, error) {
	// Check if new columns exist, if not use old schema
	var hasNewColumns bool
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('programs') WHERE name IN ('domain', 'offers_bounties', 'program_type')`).Scan(&hasNewColumns)
	if err == nil {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('programs') WHERE name IN ('domain', 'offers_bounties', 'program_type')`).Scan(&count)
		hasNewColumns = count == 3
	}

	var rows *sql.Rows
	if hasNewColumns {
		rows, err = db.Query(`SELECT id, name, handle, url, domain, offers_bounties, program_type, last_scanned FROM programs`)
	} else {
		// Fallback to old schema
		rows, err = db.Query(`SELECT id, name, handle, url, last_scanned FROM programs`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var programs []Program
	for rows.Next() {
		var p Program
		if hasNewColumns {
			if err := rows.Scan(&p.ID, &p.Name, &p.Handle, &p.URL, &p.Domain, &p.OffersBounties, &p.ProgramType, &p.LastScanned); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&p.ID, &p.Name, &p.Handle, &p.URL, &p.LastScanned); err != nil {
				return nil, err
			}
			// Set defaults for old schema
			p.Domain = ""
			p.OffersBounties = false
			p.ProgramType = "UNKNOWN"
		}
		programs = append(programs, p)
	}
	return programs, nil
}

func (db *DB) GetProgramsByType(programType string) ([]Program, error) {
	// Use COALESCE to handle missing columns gracefully
	rows, err := db.Query(`SELECT id, name, handle, url, 
		COALESCE(domain, '') as domain,
		COALESCE(offers_bounties, 0) as offers_bounties,
		COALESCE(program_type, 'UNKNOWN') as program_type,
		last_scanned 
		FROM programs WHERE COALESCE(program_type, 'UNKNOWN') = ?`, programType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var programs []Program
	for rows.Next() {
		var p Program
		if err := rows.Scan(&p.ID, &p.Name, &p.Handle, &p.URL, &p.Domain, &p.OffersBounties, &p.ProgramType, &p.LastScanned); err != nil {
			return nil, err
		}
		programs = append(programs, p)
	}
	return programs, nil
}

func (db *DB) GetProgramsWithBounties() ([]Program, error) {
	// Use COALESCE to handle missing columns gracefully
	rows, err := db.Query(`SELECT id, name, handle, url, 
		COALESCE(domain, '') as domain,
		COALESCE(offers_bounties, 0) as offers_bounties,
		COALESCE(program_type, 'UNKNOWN') as program_type,
		last_scanned 
		FROM programs WHERE COALESCE(offers_bounties, 0) = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var programs []Program
	for rows.Next() {
		var p Program
		if err := rows.Scan(&p.ID, &p.Name, &p.Handle, &p.URL, &p.Domain, &p.OffersBounties, &p.ProgramType, &p.LastScanned); err != nil {
			return nil, err
		}
		programs = append(programs, p)
	}
	return programs, nil
}

func (db *DB) SaveDomain(domain *Domain) error {
	// Check if domain already exists and get old status
	var existingID int64
	var existingIsNew bool
	var oldStatus string
	err := db.QueryRow(`SELECT id, is_new, status FROM domains WHERE domain = ? AND program = ?`,
		domain.Domain, domain.Program).Scan(&existingID, &existingIsNew, &oldStatus)

	if err == sql.ErrNoRows {
		// New domain
		query := `INSERT INTO domains (domain, program, status, discovered_at, last_checked, is_new)
		          VALUES (?, ?, ?, ?, ?, 1)`
		_, err = db.Exec(query, domain.Domain, domain.Program, domain.Status,
			domain.DiscoveredAt, domain.LastChecked)
		return err
	} else if err != nil {
		return err
	}

	// Check if status changed (especially down to up)
	if oldStatus != domain.Status {
		// Record status change (ignore errors if table doesn't exist yet)
		changeQuery := `INSERT INTO status_changes (domain, program, old_status, new_status, changed_at, notified)
		                VALUES (?, ?, ?, ?, ?, 0)`
		if _, err := db.Exec(changeQuery, domain.Domain, domain.Program, oldStatus, domain.Status, time.Now()); err != nil {
			// Table might not exist yet, that's okay
			_ = err
		}
		
		// If status changed from down to up, mark as important
		if oldStatus == "down" && domain.Status == "up" {
			log.Printf("🚨 STATUS CHANGE: %s changed from DOWN to UP in program %s", domain.Domain, domain.Program)
		}
	}

	// Update existing domain
	query := `UPDATE domains SET status = ?, last_checked = ?, is_new = ? WHERE id = ?`
	_, err = db.Exec(query, domain.Status, domain.LastChecked, false, existingID)
	return err
}

func (db *DB) GetNewDomains(limit int) ([]Domain, error) {
	rows, err := db.Query(`SELECT id, domain, program, status, discovered_at, last_checked, is_new
	                       FROM domains WHERE is_new = 1 ORDER BY discovered_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Domain, &d.Program, &d.Status, &d.DiscoveredAt, &d.LastChecked, &d.IsNew); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, nil
}

func (db *DB) GetDomainsByProgram(program string, limit int) ([]Domain, error) {
	rows, err := db.Query(`SELECT id, domain, program, status, discovered_at, last_checked, is_new
	                       FROM domains WHERE program = ? ORDER BY discovered_at DESC LIMIT ?`, program, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Domain, &d.Program, &d.Status, &d.DiscoveredAt, &d.LastChecked, &d.IsNew); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, nil
}

func (db *DB) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total domains
	var totalDomains int
	if err := db.QueryRow(`SELECT COUNT(*) FROM domains`).Scan(&totalDomains); err != nil {
		return nil, err
	}
	stats["total_domains"] = totalDomains

	// New domains
	var newDomains int
	if err := db.QueryRow(`SELECT COUNT(*) FROM domains WHERE is_new = 1`).Scan(&newDomains); err != nil {
		return nil, err
	}
	stats["new_domains"] = newDomains

	// Up domains
	var upDomains int
	if err := db.QueryRow(`SELECT COUNT(*) FROM domains WHERE status = 'up'`).Scan(&upDomains); err != nil {
		return nil, err
	}
	stats["up_domains"] = upDomains

	// Down domains
	var downDomains int
	if err := db.QueryRow(`SELECT COUNT(*) FROM domains WHERE status = 'down'`).Scan(&downDomains); err != nil {
		return nil, err
	}
	stats["down_domains"] = downDomains

	// Total programs
	var totalPrograms int
	if err := db.QueryRow(`SELECT COUNT(*) FROM programs`).Scan(&totalPrograms); err != nil {
		return nil, err
	}
	stats["total_programs"] = totalPrograms

	return stats, nil
}

func (db *DB) MarkDomainsAsOld() error {
	_, err := db.Exec(`UPDATE domains SET is_new = 0 WHERE is_new = 1`)
	return err
}

func (db *DB) GetStatusChanges(limit int, onlyUnnotified bool) ([]StatusChange, error) {
	// Check if status_changes table exists
	var tableExists int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='status_changes'`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		// Table doesn't exist yet, return empty
		return []StatusChange{}, nil
	}

	var query string
	if onlyUnnotified {
		query = `SELECT id, domain, program, old_status, new_status, changed_at, notified
		         FROM status_changes WHERE notified = 0 ORDER BY changed_at DESC LIMIT ?`
	} else {
		query = `SELECT id, domain, program, old_status, new_status, changed_at, notified
		         FROM status_changes ORDER BY changed_at DESC LIMIT ?`
	}

	rows, err := db.Query(query, limit)
	if err != nil {
		return []StatusChange{}, nil // Return empty instead of error
	}
	defer rows.Close()

	var changes []StatusChange
	for rows.Next() {
		var sc StatusChange
		if err := rows.Scan(&sc.ID, &sc.Domain, &sc.Program, &sc.OldStatus, &sc.NewStatus, &sc.ChangedAt, &sc.Notified); err != nil {
			return nil, err
		}
		changes = append(changes, sc)
	}
	return changes, nil
}

func (db *DB) MarkStatusChangeNotified(id int64) error {
	_, err := db.Exec(`UPDATE status_changes SET notified = 1 WHERE id = ?`, id)
	return err
}

func (db *DB) SaveDomainInfo(info *DomainInfo) error {
	techsStr := strings.Join(info.Technologies, ",")
	query := `INSERT OR REPLACE INTO domain_info (domain, program, status, title, status_code, technologies, last_checked, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, info.Domain, info.Program, info.Status, info.Title, 
		info.StatusCode, techsStr, info.LastChecked, time.Now())
	return err
}

func (db *DB) GetDomainInfo(domain string) (*DomainInfo, error) {
	var info DomainInfo
	var techsStr string
	err := db.QueryRow(`SELECT domain, program, status, title, status_code, technologies, last_checked
	                    FROM domain_info WHERE domain = ?`, domain).
		Scan(&info.Domain, &info.Program, &info.Status, &info.Title, 
			&info.StatusCode, &techsStr, &info.LastChecked)
	if err != nil {
		return nil, err
	}
	if techsStr != "" {
		info.Technologies = strings.Split(techsStr, ",")
	}
	return &info, nil
}
