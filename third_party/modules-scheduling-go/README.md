# modules/scheduling/go

A reusable scheduling and virtual queue module:

- branch, service, and resource catalog
- availability rules and blocked ranges
- bookings with holds, confirmation, cancellation, rescheduling, check-in, and completion flows
- queue tickets with issue, call, serve, return-to-waiting, complete, and no-show flows
- waitlist entries and booking action tokens
- GORM adapter for PostgreSQL
- reusable `migrations` package for the module schema
- reusable `seeds` package for demo bootstrap data
- reusable `httpgin` adapter for authenticated Gin APIs
- reusable `publichttpgin` adapter for public booking and queue APIs

It consumes `core/scheduling/go` primitives for slot generation and time window handling.
