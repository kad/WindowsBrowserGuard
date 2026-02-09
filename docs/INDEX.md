# Windows Browser Guard Documentation Index

This directory contains all documentation for Windows Browser Guard, organized by purpose.

## 📚 Quick Links

- **[README.md](README.md)** - Main user guide and installation instructions
- **[Project Summary](../PROJECT-SUMMARY.md)** - High-level project overview (in root)

## 📁 Documentation Structure

### `/features` - Feature Documentation
Detailed documentation for each major feature of the application.

- **[OPENTELEMETRY.md](features/OPENTELEMETRY.md)** - OpenTelemetry integration overview
- **[OPENTELEMETRY-LOGGING.md](features/OPENTELEMETRY-LOGGING.md)** - Structured logging with OTLP export
- **[OPENTELEMETRY-METRICS.md](features/OPENTELEMETRY-METRICS.md)** - Metrics collection and export
- **[OTLP-ENDPOINTS.md](features/OTLP-ENDPOINTS.md)** - OTLP endpoint configuration (gRPC/HTTP)
- **[DRY-RUN-MODE.md](features/DRY-RUN-MODE.md)** - Testing mode without system modifications

### `/development` - Development History & Implementation Summaries
Historical documents tracking the evolution of the codebase.

**Architecture & Refactoring:**
- **[MAIN-REFACTORING.md](development/MAIN-REFACTORING.md)** - Initial code organization refactoring
- **[RESTRUCTURE.md](development/RESTRUCTURE.md)** - Project structure reorganization to pkg/ and cmd/
- **[CLEANUP-COMPLETE.md](development/CLEANUP-COMPLETE.md)** - Cleanup completion summary
- **[REFACTORING-COMPLETE.md](development/REFACTORING-COMPLETE.md)** - Final refactoring summary

**Feature Implementation:**
- **[detection-module.md](development/detection-module.md)** - Extension detection logic development
- **[LOGGING-INTEGRATION.md](development/LOGGING-INTEGRATION.md)** - OpenTelemetry logging implementation
- **[METRICS-INTEGRATION.md](development/METRICS-INTEGRATION.md)** - OpenTelemetry metrics implementation
- **[DOCS-SCRIPTS-UPDATE.md](development/DOCS-SCRIPTS-UPDATE.md)** - Documentation and scripts update

**Testing & Debugging:**
- **[DEBUG-CLEANUP.md](development/DEBUG-CLEANUP.md)** - Debug cleanup and log optimization
- **[TEST-VERIFICATION.md](development/TEST-VERIFICATION.md)** - Testing procedures and verification
- **[TASK-SCHEDULER-FIX.md](development/TASK-SCHEDULER-FIX.md)** - Task scheduler installation fix for auto-start issues

**Build & Release:**
- **[GORELEASER.md](development/GORELEASER.md)** - GoReleaser configuration and release process

### `/guides` - User Guides & How-Tos
Step-by-step guides for common tasks.

- **[INSTALLATION.md](guides/INSTALLATION.md)** - Complete installation guide with installer script
- **[MAINTENANCE-SCRIPTS.md](guides/MAINTENANCE-SCRIPTS.md)** - Complete guide to maintenance scripts

## 🛠️ PowerShell Scripts

These scripts are located in the `/docs` directory root:

**Installation:**
- **[install-task.ps1](install-task.ps1)** - Install as Windows scheduled task with OTLP configuration
- **[uninstall-task.ps1](uninstall-task.ps1)** - Remove scheduled task and clean up files

**Maintenance:**
- **[start.ps1](start.ps1)** - Start the monitor (task scheduler or direct mode)
- **[stop.ps1](stop.ps1)** - Stop the monitor
- **[restart.ps1](restart.ps1)** - Restart the monitor
- **[status.ps1](status.ps1)** - Show comprehensive status (process, task, logs, config)

**Monitoring:**
- **[view-logs.ps1](view-logs.ps1)** - View and tail log files

**Legacy:**
- **[start-monitor.ps1](start-monitor.ps1)** - Original manual start script (use `start.ps1` instead)

📖 **Complete guide:** [guides/MAINTENANCE-SCRIPTS.md](guides/MAINTENANCE-SCRIPTS.md)

## 📖 Reading Order for New Developers

1. **[../PROJECT-SUMMARY.md](../PROJECT-SUMMARY.md)** - Understand what the project does
2. **[guides/INSTALLATION.md](guides/INSTALLATION.md)** - Learn how to install
3. **[README.md](README.md)** - Learn how to use
4. **[guides/MAINTENANCE-SCRIPTS.md](guides/MAINTENANCE-SCRIPTS.md)** - Learn the maintenance scripts
5. **[features/DRY-RUN-MODE.md](features/DRY-RUN-MODE.md)** - Test without system changes
6. **[features/OPENTELEMETRY.md](features/OPENTELEMETRY.md)** - Observability features overview
7. **[development/RESTRUCTURE.md](development/RESTRUCTURE.md)** - Understand code organization

## 🔍 Finding Documentation

**Want to know about a specific feature?**
→ Check `/features` directory

**Need to understand how something was implemented?**
→ Check `/development` directory

**Want to install or use the application?**
→ Read `README.md`

**Looking for troubleshooting?**
→ Check `README.md` troubleshooting section

---

*Last updated: 2026-02-09*
