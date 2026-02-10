# Deployment Guide for Hetzner Server

This guide walks you through deploying the Episod to a Hetzner server with Docker, Nginx, and SSL.

## Prerequisites

On your Hetzner server:
- Ubuntu 20.04+ (or similar Linux distribution)
- Docker and Docker Compose installed
- FFmpeg installed
- A domain name pointing to your server's IP address
- Port 80 and 443 open in firewall

## Step 1: Push Code to Git Repository

From your local machine:

```bash
# Make sure all changes are committed
git add .
git commit -m "Prepare for deployment"

# Push to your repository (GitHub/GitLab)
git push origin main
```

## Step 2: Connect to Your Hetzner Server

```bash
ssh root@YOUR_SERVER_IP
```

## Step 3: Install Prerequisites (if not already installed)

```bash
# Update system
apt update && apt upgrade -y

# Install required packages
apt install -y nginx certbot python3-certbot-nginx git curl

# Verify Docker is installed
docker --version
docker compose version

# Verify FFmpeg is installed
ffmpeg -version
```

## Step 4: Clone the Repository on Server

```bash
# Navigate to your desired directory
cd /opt

# Clone your repository
git clone YOUR_REPOSITORY_URL episod
cd episod
```

## Step 5: Configure Environment Variables

```bash
# Create production .env file
nano .env
```

Add the following (replace with your actual values):

```bash
# Server Configuration
API_PORT=8080
WORKER_ENABLED=true

# Database Password (IMPORTANT: Use a strong password)
DB_PASSWORD=your_strong_database_password_here

# API Security
BACKEND_API_KEY=your-secure-api-key-here

# CORS - Add your domain
CORS_ALLOWED_ORIGINS=https://yourdomain.com

# Database (using Docker internal network)
DATABASE_URL=postgresql://postgres:your_strong_database_password_here@postgres:5432/faceless?sslmode=disable

# Redis
REDIS_URL=redis://redis:6379

# Supabase
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_KEY=your-service-key
SUPABASE_STORAGE_BUCKET=files

# OpenAI
OPENAI_API_KEY=your-openai-key

# Gemini
GEMINI_API_KEY=your-gemini-key

# TTS - ElevenLabs (recommended)
ELEVENLABS_API_KEY=your-elevenlabs-key
# Optional: ELEVENLABS_VOICE_ID=pNInz6obpgDQGcFmaJgB

# Alternative TTS - Cartesia (if not using ElevenLabs)
# CARTESIA_API_KEY=your-cartesia-key

# Optional: xAI Video Generation
# XAI_VIDEO_ENABLED=true
# XAI_API_KEY=your-xai-api-key

# Optional: Veo Video Generation
# VEO_ENABLED=true

# Worker Configuration
MAX_CONCURRENT_JOBS=3
```

Save and exit (Ctrl+X, Y, Enter).

**IMPORTANT**: Secure the .env file:
```bash
chmod 600 .env
```

## Step 6: Build and Start Docker Containers

```bash
# Build and start services using production compose file
docker compose -f docker-compose.prod.yml up -d --build

# Check if containers are running
docker compose -f docker-compose.prod.yml ps

# View logs to ensure everything started correctly
docker compose -f docker-compose.prod.yml logs -f
```

Press Ctrl+C to exit logs when satisfied.

## Step 7: Configure Nginx Reverse Proxy

```bash
# Copy the nginx template
cp nginx.conf.template /etc/nginx/sites-available/episod

# Replace YOUR_DOMAIN with your actual domain
sed -i 's/YOUR_DOMAIN/yourdomain.com/g' /etc/nginx/sites-available/episod

# Or edit manually if you prefer
nano /etc/nginx/sites-available/episod
```

Enable the site (before SSL):
```bash
# Create a temporary HTTP-only config for Certbot
cat > /etc/nginx/sites-available/episod-temp << 'EOF'
server {
    listen 80;
    listen [::]:80;
    server_name yourdomain.com;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF

# Replace yourdomain.com with your domain
sed -i 's/yourdomain.com/YOUR_ACTUAL_DOMAIN/g' /etc/nginx/sites-available/episod-temp

# Enable temporary config
ln -sf /etc/nginx/sites-available/episod-temp /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

# Test nginx configuration
nginx -t

# Reload nginx
systemctl reload nginx
```

## Step 8: Obtain SSL Certificate

```bash
# Create directory for certbot
mkdir -p /var/www/certbot

# Obtain SSL certificate
certbot --nginx -d yourdomain.com

# Follow the prompts:
# - Enter your email
# - Agree to terms
# - Choose whether to redirect HTTP to HTTPS (recommended: Yes)
```

## Step 9: Switch to Full Nginx Configuration

```bash
# Remove temporary config
rm /etc/nginx/sites-enabled/episod-temp

# Enable full config with SSL
ln -sf /etc/nginx/sites-available/episod /etc/nginx/sites-enabled/

# Test configuration
nginx -t

# Reload nginx
systemctl reload nginx
```

## Step 10: Verify Deployment

```bash
# Check if API is accessible locally
curl http://localhost:8080/health

# Check via domain
curl https://yourdomain.com/health

# Should return: {"status":"ok"}
```

## Step 11: Test the API

```bash
# Create a test project (replace YOUR_API_KEY with your BACKEND_API_KEY)
curl -X POST https://yourdomain.com/v1/projects \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_API_KEY" \
  -d '{
    "topic": "The History of Coffee",
    "target_duration_seconds": 30
  }'

# Note the project_id from response, then check status:
curl https://yourdomain.com/v1/projects/PROJECT_ID \
  -H "X-API-Key: YOUR_API_KEY"
```

## Useful Management Commands

### View Logs
```bash
# All services
docker compose -f docker-compose.prod.yml logs -f

# Specific service
docker compose -f docker-compose.prod.yml logs -f api
docker compose -f docker-compose.prod.yml logs -f postgres
docker compose -f docker-compose.prod.yml logs -f redis
```

### Restart Services
```bash
# Restart all
docker compose -f docker-compose.prod.yml restart

# Restart specific service
docker compose -f docker-compose.prod.yml restart api
```

### Update Deployment
```bash
cd /opt/episod

# Pull latest code
git pull

# Rebuild and restart
docker compose -f docker-compose.prod.yml up -d --build

# Or rebuild specific service
docker compose -f docker-compose.prod.yml up -d --build api
```

### Stop Services
```bash
docker compose -f docker-compose.prod.yml down
```

### Stop and Remove Everything (including data)
```bash
docker compose -f docker-compose.prod.yml down -v
```

## Monitoring

### Check Resource Usage
```bash
# Docker stats
docker stats

# Disk usage
df -h

# Database size
docker compose -f docker-compose.prod.yml exec postgres psql -U postgres -d faceless -c "SELECT pg_size_pretty(pg_database_size('faceless'));"
```

### Nginx Logs
```bash
# Access log
tail -f /var/log/nginx/episod_access.log

# Error log
tail -f /var/log/nginx/episod_error.log
```

## Troubleshooting

### API not responding
```bash
# Check if containers are running
docker compose -f docker-compose.prod.yml ps

# Check API logs
docker compose -f docker-compose.prod.yml logs api

# Check if port is accessible locally
curl http://localhost:8080/health
```

### Database connection issues
```bash
# Check database logs
docker compose -f docker-compose.prod.yml logs postgres

# Verify database is ready
docker compose -f docker-compose.prod.yml exec postgres pg_isready -U postgres
```

### Worker not processing jobs
```bash
# Check worker logs in API container
docker compose -f docker-compose.prod.yml logs api | grep -i worker

# Check Redis connection
docker compose -f docker-compose.prod.yml exec redis redis-cli ping
```

### SSL certificate renewal
```bash
# Certbot auto-renews, but you can test renewal:
certbot renew --dry-run

# Force renewal (if needed)
certbot renew --force-renewal
systemctl reload nginx
```

## Security Recommendations

1. **Firewall Configuration**:
```bash
# Install UFW if not already installed
apt install ufw

# Allow SSH (important - don't lock yourself out!)
ufw allow 22/tcp

# Allow HTTP and HTTPS
ufw allow 80/tcp
ufw allow 443/tcp

# Enable firewall
ufw enable
```

2. **Keep API Key Secure**: Never share your `BACKEND_API_KEY` or commit it to public repositories.

3. **Regular Updates**:
```bash
# Update system packages regularly
apt update && apt upgrade -y

# Update Docker images periodically
cd /opt/episod
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

4. **Backup Database**:
```bash
# Create backup script
cat > /opt/episod/backup.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/opt/episod/backups"
mkdir -p $BACKUP_DIR
DATE=$(date +%Y%m%d_%H%M%S)
docker compose -f /opt/episod/docker-compose.prod.yml exec -T postgres pg_dump -U postgres faceless | gzip > $BACKUP_DIR/backup_$DATE.sql.gz
# Keep only last 7 backups
ls -t $BACKUP_DIR/backup_*.sql.gz | tail -n +8 | xargs -r rm
EOF

chmod +x /opt/episod/backup.sh

# Add to crontab for daily backups at 2 AM
(crontab -l 2>/dev/null; echo "0 2 * * * /opt/episod/backup.sh") | crontab -
```

## Support

For issues and questions, refer to the main README.md or open an issue on GitHub.
