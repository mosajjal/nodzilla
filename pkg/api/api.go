package api

import (
	"crypto/subtle"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mosajjal/nodzilla/pkg/db"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

// API is the main struct for the API. can be used for both Admin and Query APIs
type API struct {
	*echo.Echo
	DB          db.NodDB
	middlewares []echo.MiddlewareFunc
	C           Config
}

// Config struct is corresponding to the YAML payload in api section of the config file
type Config struct {
	// BasePath is the basepath for the API
	BasePath string
	// BasePathAdmin is the basepath for the Admin API
	BasePathAdmin string
	// ListenAddr is the address to listen on
	ListenAddr string
	// IsTLS is a flag to enable TLS
	IsTLS bool
	// TLSCert is the path to the TLS certificate
	TLSCert string
	// TLSKey is the path to the TLS key
	TLSKey string
	// AuthMethodAPI is the authentication method to use. Can be "none" or "basic"
	AuthMethodAPI string
	// AuthUsersAPI is a map of username:password for basic auth
	AuthUsersAPI map[string]string
	// AuthMethodAdmin is the authentication method to use. Can be "none" or "basic"
	AuthMethodAdmin string
	// AuthUsersAdmin is a map of username:password for basic auth
	AuthUsersAdmin map[string]string
	// Logger is the logger to use
	Logger *zerolog.Logger
	// RPS is the rate limit for the API
	RPS float64
}

// NewAPI creates a new API instance. It won't start till ListenAndServe is called
func NewAPI(config Config, db db.NodDB) *API {
	// Create a new Echo instance
	e := echo.New()
	// Create a new API instance
	api := &API{
		Echo: e,
		DB:   db,
		C:    config,
	}
	// Add the middlewares
	api.addMiddlewares()
	// Add the query paths
	api.AddQueryPaths()
	// Add the admin paths
	api.AddAdminPaths()
	// Return the API instance
	return api
}

// ListenAndServe starts the API server based on the config
func (api *API) ListenAndServe() {
	if api.C.IsTLS {
		api.Logger.Fatal(api.StartTLS(api.C.ListenAddr, api.C.TLSCert, api.C.TLSKey))
	} else {
		api.Logger.Fatal(api.Start(api.C.ListenAddr))
	}
}

func (api *API) addMiddlewares() {
	// the middlewares seems to be in order. so we'll log first, then rate limit, then auth
	// set up a logger
	api.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			api.C.Logger.Info().
				Str("URI", v.URI).
				Int("status", v.Status).
				Msg("request")
			return nil
		},
	}))

	// the following rate limit restricts the number of requests to 1 per second for an IP
	// TODO: make this configurable and more flexible
	api.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(api.C.RPS))))

	// add auth middleware if auth is enabled
	if api.C.AuthMethodAPI != "none" || api.C.AuthMethodAdmin != "none" {
		api.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			// depending on URL being on admin prefix or not, use the appropriate auth method
			// check authentication schemme for admin base path
			if strings.HasPrefix(c.Path(), api.C.BasePathAdmin) {
				if api.C.AuthMethodAdmin == "none" {
					return true, nil
				}
				for user, pass := range api.C.AuthUsersAdmin {
					if subtle.ConstantTimeCompare([]byte(username), []byte(user)) == 1 &&
						subtle.ConstantTimeCompare([]byte(password), []byte(pass)) == 1 {
						return true, nil
					}
				}
				return false, nil
			}
			// check authentication schemme for API base path
			if strings.HasPrefix(c.Path(), api.C.BasePath) || c.Path() == "" {
				if api.C.AuthMethodAPI == "none" {
					return true, nil
				}
				for user, pass := range api.C.AuthUsersAPI {
					if subtle.ConstantTimeCompare([]byte(username), []byte(user)) == 1 &&
						subtle.ConstantTimeCompare([]byte(password), []byte(pass)) == 1 {
						return true, nil
					}
				}
			}
			return false, nil
		}))
	}
}

// AddQueryPaths adds the two query URLs to the Echo instance
func (api *API) AddQueryPaths() {
	// query path is meant to be used by a real user through a browser
	// so it doesn't require a JSON payload
	api.GET(api.C.BasePath+"query/:domain", api.query, api.middlewares...)
	// query_many path gets a list of domains as payload and returns a JSON object
	// for each domain and it's respective registration date
	// $ curl -XGET http://127.0.0.1:3000/query_many -H 'Content-Type: application/json' -d '["domain1.com","domain2.com"]'
	// > {"domain1.com":"2020-01-01T12:00:00+12:00", "domain2.com": "2020-01-01T12:00:00+12:00"}
	api.GET(api.C.BasePath+"query_many", api.queryMany, api.middlewares...)
}

// AddAdminPaths adds the admin URLs to the Echo instance
func (api *API) AddAdminPaths() {
	// add_domain path is used to add a new domain to the database
	api.POST(api.C.BasePathAdmin+"/add_domain", api.addDomain, api.middlewares...)
	// add_domains path is used to add a list of domains to the database
	api.POST(api.C.BasePathAdmin+"/add_domains", api.addDomains, api.middlewares...)
	// delete_domain path is used to delete a domain from the database
	api.DELETE(api.C.BasePathAdmin+"/delete_domain/:domain", api.deleteDomain, api.middlewares...)
	// delete_domains path is used to delete a list of domains from the database
	api.DELETE(api.C.BasePathAdmin+"/delete_domains", api.deleteDomains, api.middlewares...)
}

func (api *API) query(c echo.Context) error {
	// Get the domain from the URL
	domain := c.Param("domain")
	// Query the database for the domain
	// If the domain is not found, return a 404
	// If the domain is found, return a JSON object with the domain and it's registration date
	entry, err := api.DB.Query(domain)
	if err != nil {
		return c.JSON(500, map[string]string{"error": "internal server error"})
	}
	if entry.Domain == "" {
		return c.JSON(404, map[string]string{"error": "domain not found"})
	}
	return c.JSON(200, map[string]string{"domain": entry.Domain, "registration_date": entry.RegistrationDate.Format(time.RFC3339)})
}

func (api *API) queryMany(c echo.Context) error {
	// Get the list of domains from the JSON payload
	var domains []string
	if err := c.Bind(&domains); err != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	// Query the database for each domain
	// If the domain is not found, return a 404
	// If the domain is found, return a JSON object with the domain and it's registration date
	entries, err := api.DB.QueryMany(domains)
	if err != nil {
		return c.JSON(500, map[string]string{"error": "internal server error"})
	}
	return c.JSON(200, entries)

}

// addDomain adds a single domain to the database
// request body should be a JSON string with the domain and it's registration date
// $ curl -XPOST http://127.0.0.1:3000 -H 'Content-Type: application/json' -d '{"domain":"domain.com","registration_date":"2020-01-01T12:00:00+12:00"}'
func (api *API) addDomain(c echo.Context) error {
	// Get the domain and registration date from the JSON payload
	var entry db.Entry
	if err := c.Bind(&entry); err != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	// Add the domain to the database
	// If the domain is not added successfully, return a 409
	// If the domain is added successfully, return a 200
	err := api.DB.Add(entry)
	if err != nil {
		return c.JSON(409, map[string]string{"error": "insert failed"})
	}
	return c.JSON(200, map[string]string{"status": "ok"})
}

// addDomains adds a list of domains to the database
// request body should be a JSON array of JSON objects with the domain and it's registration date
func (api *API) addDomains(c echo.Context) error {
	// Get the list of domains from the JSON payload
	var entries []db.Entry
	if err := c.Bind(&entries); err != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	// Add the domains to the database
	// if the domain is not added successfully, return a 409
	// If the domain is added successfully, return a 200
	err := api.DB.AddMany(entries)
	if err != nil {
		return c.JSON(409, map[string]string{"error": "insert failed"})
	}
	return c.JSON(200, map[string]string{"status": "ok"})
}

func (api *API) deleteDomain(c echo.Context) error {
	// Get the domain from the URL
	domain := c.Param("domain")
	// Delete the domain from the database
	// If the domain is not found, return a 404
	// If the domain is deleted successfully, return a 200
	err := api.DB.Delete(domain)
	if err != nil {
		return c.JSON(404, map[string]string{"error": "domain not found"})
	}
	return c.JSON(200, map[string]string{"status": "ok"})
}

func (api *API) deleteDomains(c echo.Context) error {
	// Get the list of domains from the JSON payload
	var domains []string
	if err := c.Bind(&domains); err != nil {
		return c.JSON(400, map[string]string{"error": "bad request"})
	}
	// Delete the domains from the database
	// If the domain is not found, return a 404
	// If the domain is deleted successfully, return a 200
	err := api.DB.DeleteMany(domains)
	if err != nil {
		return c.JSON(404, map[string]string{"error": "domain not found"})
	}
	return c.JSON(200, map[string]string{"status": "ok"})
}
