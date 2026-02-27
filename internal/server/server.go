package server

import (
	"net/http"
	"strconv"

	"watchtower/internal/database"

	"github.com/gin-gonic/gin"
)

type Server struct {
	db   *database.DB
	port string
}

func NewServer(db *database.DB, port string) *Server {
	return &Server{
		db:   db,
		port: port,
	}
}

func (s *Server) Start() error {
	router := gin.Default()

	// Serve static files and HTML
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// API routes
	api := router.Group("/api/v1")
	{
		api.GET("/stats", s.getStats)
		api.GET("/domains/new", s.getNewDomains)
		api.GET("/domains", s.getDomains)
		api.GET("/domains/program/:program", s.getDomainsByProgram)
		api.GET("/programs", s.getPrograms)
		api.GET("/programs/rdp", s.getRDPPrograms)
		api.GET("/programs/vdp", s.getVDPPrograms)
		api.GET("/programs/bounties", s.getBountyPrograms)
		api.GET("/status-changes", s.getStatusChanges)
		api.GET("/status-changes/unnotified", s.getUnnotifiedStatusChanges)
	}

	// Web routes
	router.GET("/", s.index)
	router.GET("/domains", s.domainsPage)
	router.GET("/programs", s.programsPage)
	router.GET("/status-changes", s.statusChangesPage)
	router.GET("/filters", s.filtersPage)

	return router.Run(":" + s.port)
}

func (s *Server) getStats(c *gin.Context) {
	stats, err := s.db.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) getNewDomains(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	domains, err := s.db.GetNewDomains(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, domains)
}

func (s *Server) getDomains(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	program := c.Query("program")
	if program != "" {
		domains, err := s.db.GetDomainsByProgram(program, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, domains)
		return
	}

	// Get new domains by default
	domains, err := s.db.GetNewDomains(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, domains)
}

func (s *Server) getDomainsByProgram(c *gin.Context) {
	program := c.Param("program")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	domains, err := s.db.GetDomainsByProgram(program, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, domains)
}

func (s *Server) getPrograms(c *gin.Context) {
	programs, err := s.db.GetPrograms()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, programs)
}

func (s *Server) index(c *gin.Context) {
	stats, _ := s.db.GetStats()
	newDomains, _ := s.db.GetNewDomains(10)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Stats":      stats,
		"NewDomains": newDomains,
	})
}

func (s *Server) domainsPage(c *gin.Context) {
	program := c.Query("program")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	var domains []database.Domain
	var err error

	if program != "" {
		domains, err = s.db.GetDomainsByProgram(program, limit)
	} else {
		domains, err = s.db.GetNewDomains(limit)
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Error": err.Error(),
		})
		return
	}

	programs, _ := s.db.GetPrograms()

	c.HTML(http.StatusOK, "domains.html", gin.H{
		"Domains":         domains,
		"Programs":        programs,
		"SelectedProgram": program,
	})
}

func (s *Server) programsPage(c *gin.Context) {
	programType := c.Query("type")
	bountiesOnly := c.Query("bounties") == "true"

	var programs []database.Program
	var err error

	if programType == "RDP" {
		programs, err = s.db.GetProgramsByType("RDP")
	} else if programType == "VDP" {
		programs, err = s.db.GetProgramsByType("VDP")
	} else if bountiesOnly {
		programs, err = s.db.GetProgramsWithBounties()
	} else {
		programs, err = s.db.GetPrograms()
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Error": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "programs.html", gin.H{
		"Programs":    programs,
		"ProgramType": programType,
		"BountiesOnly": bountiesOnly,
	})
}

func (s *Server) getRDPPrograms(c *gin.Context) {
	programs, err := s.db.GetProgramsByType("RDP")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, programs)
}

func (s *Server) getVDPPrograms(c *gin.Context) {
	programs, err := s.db.GetProgramsByType("VDP")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, programs)
}

func (s *Server) getBountyPrograms(c *gin.Context) {
	programs, err := s.db.GetProgramsWithBounties()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, programs)
}

func (s *Server) getStatusChanges(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	changes, err := s.db.GetStatusChanges(limit, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, changes)
}

func (s *Server) getUnnotifiedStatusChanges(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	changes, err := s.db.GetStatusChanges(limit, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, changes)
}

func (s *Server) statusChangesPage(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	changes, err := s.db.GetStatusChanges(limit, false)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Error": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "status-changes.html", gin.H{
		"StatusChanges": changes,
	})
}

func (s *Server) filtersPage(c *gin.Context) {
	stats, _ := s.db.GetStats()
	rdpPrograms, _ := s.db.GetProgramsByType("RDP")
	vdpPrograms, _ := s.db.GetProgramsByType("VDP")
	bountyPrograms, _ := s.db.GetProgramsWithBounties()

	c.HTML(http.StatusOK, "filters.html", gin.H{
		"Stats":         stats,
		"RDPPrograms":   rdpPrograms,
		"VDPPrograms":   vdpPrograms,
		"BountyPrograms": bountyPrograms,
	})
}
