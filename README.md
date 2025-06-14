# Archivus

**AI-Powered Document Management System (DMS) Built in Go**

Archivus is a next-generation document management system that leverages the performance of Go and the intelligence of modern AI to help small and medium-sized businesses manage, organize, and automate their documents with ease.

---

## 🚀 Quick Start

### Prerequisites
- Go 1.21 or higher
- PostgreSQL 16+ with pgvector extension
- Redis 7+
- Docker & Docker Compose (optional)

### Development Setup

1. **Clone and setup environment**
```bash
git clone <repository-url>
cd archivus
make setup-dev  # Copies env.example to .env
```

2. **Configure environment**
Edit `.env` file with your database and API credentials:
```bash
DATABASE_URL=postgres://username:password@localhost:5432/archivus_dev?sslmode=disable
JWT_SECRET=your-super-secret-jwt-key
OPENAI_API_KEY=your-openai-api-key  # Optional for AI features
```

3. **Start with Docker (Recommended)**
```bash
make docker-dev  # Starts PostgreSQL, Redis, and the app
```

4. **Or run locally**
```bash
make deps        # Install dependencies
make db-setup-dev # Create databases
make migrate-up  # Run migrations
make run         # Start the server
```

5. **Verify installation**
```bash
curl http://localhost:8080/health
```

---

## 📁 Project Structure

```
archivus/
├── cmd/                    # Application entry points
│   ├── server/            # Main server application
│   └── migrate/           # Database migration tool
├── internal/              # Private application code
│   ├── app/              # Application layer
│   │   ├── config/       # Configuration management
│   │   ├── handlers/     # HTTP handlers
│   │   ├── middleware/   # HTTP middleware
│   │   └── server/       # Server setup
│   ├── domain/           # Business logic layer
│   │   ├── entities/     # Domain entities
│   │   ├── repositories/ # Data access interfaces
│   │   └── services/     # Business services
│   └── infrastructure/   # External concerns
│       ├── database/     # Database connections
│       ├── storage/      # File storage
│       ├── ai/          # AI service integrations
│       └── cache/       # Caching layer
├── pkg/                  # Public packages
│   ├── logger/          # Structured logging
│   ├── utils/           # Utility functions
│   └── validator/       # Input validation
├── api/                 # API documentation
├── web/                 # Static files and templates
├── scripts/             # Build and deployment scripts
└── deployments/         # Docker and deployment configs
```

---

## 🛠️ Available Commands

```bash
make help           # Show all available commands
make deps           # Install dependencies
make run            # Run the application
make test           # Run tests
make build          # Build for production
make docker-run     # Run with Docker Compose
make migrate-up     # Run database migrations
make lint           # Run code linter
make fmt            # Format code
```

---

## 🔧 Development

### Environment Configuration
The application supports three environments:
- `development` - Local development with debug logging
- `test` - Testing environment with test database
- `production` - Production environment with optimized settings

### Database Migrations
```bash
make migrate-create NAME=create_users_table  # Create new migration
make migrate-up                               # Apply migrations
make migrate-down                            # Rollback migrations
```

### Testing
```bash
make test              # Run all tests
make test-coverage     # Run tests with coverage report
```

---

## ✨ Key Features

- **Intelligent Document Processing**  
  Auto-tagging, summarization, entity extraction, and duplicate detection.

- **Advanced Search & Retrieval**  
  Semantic search, natural language queries, smart filters, and cross-document insights.

- **Workflow Automation**  
  Agentic AI capabilities, smart routing, compliance rule enforcement, and webhook integration.

- **Collaboration & Mobility**  
  Real-time sharing, mobile access, offline sync, and social-style document interactions.

---

## 🛠️ Tech Stack

**Backend**  
- Language: Go 1.21
- Framework: Gin
- Database: PostgreSQL 16+ with pgvector
- Caching: Redis 7+
- Authentication: JWT with MFA

**AI Layer**  
- OpenAI API (primary)
- Ollama (local alternative)
- Vector embeddings with pgvector

**Infrastructure**
- Docker & Docker Compose
- Multi-stage builds for production
- Health checks and graceful shutdown

---

## 🚀 Deployment

### Docker Production
```bash
make docker-build
docker run -p 8080:8080 --env-file .env archivus:latest
```

### Build for Different Platforms
```bash
make build-all    # Build for Linux, Windows, and macOS
```

---

## 📦 API Documentation

Once running, API documentation is available at:
- Health Check: `GET /health`
- System Status: `GET /api/v1/status`
- Full API docs: `GET /api/docs` (coming soon)

---

## 💡 Use Cases

- Legal and financial firms needing AI-driven document indexing  
- SMBs looking to modernize compliance workflows  
- Agencies managing high document volumes with team collaboration  
- Startups requiring simple, fast, secure document infrastructure  

---

## 📦 Subscription Tiers

| Tier        | Documents | Storage | AI Features | Support          |
|-------------|-----------|---------|-------------|------------------|
| Starter     | 1,000     | 5 GB    | Basic       | Email            |
| Professional| 10,000    | 50 GB   | Advanced    | Priority         |
| Enterprise  | Unlimited | Unlimited| Custom      | Dedicated        |

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

---

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.


