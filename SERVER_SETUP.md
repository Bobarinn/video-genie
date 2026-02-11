# Server Setup Guide for episod

This guide is specifically for your Hetzner server setup at `~/apps/episod` with a sudo user.

## Current Status

- ✅ Repository cloned at: `~/apps/episod`
- ✅ Git installed
- ✅ Docker installed
- ✅ FFmpeg installed
- 📦 Repository URL: `https://github.com/Bobarinn/episod`

## Step 1: Pull Latest Changes

You've cloned an older commit. Let's get the latest deployment files:

```bash
cd ~/apps/episod
git pull origin main
```

Verify the deployment files are present:
```bash
ls -la deploy.sh docker-compose.prod.yml nginx.conf.template
```

You should see all three files listed.

## Step 2: Create Environment Configuration

Create your `.env` file with all your API keys and configuration:

```bash
cd ~/apps/episod
nano .env
```

**Copy and paste this template, then fill in your actual values:**

```bash
# Server Configuration
API_PORT=8080
WORKER_ENABLED=true

# Database Password (IMPORTANT: Use a strong password!)
DB_PASSWORD=CHANGE_THIS_TO_A_STRONG_PASSWORD

# API Security (This key protects your API endpoints)
BACKEND_API_KEY=CHANGE_THIS_TO_YOUR_SECURE_API_KEY

# CORS - Add your domain (or leave empty for development)
CORS_ALLOWED_ORIGINS=https://yourdomain.com

# Database URL (uses Docker internal network)
DATABASE_URL=postgresql://postgres:YOUR_DB_PASSWORD_HERE@postgres:5432/faceless?sslmode=disable

# Redis
REDIS_URL=redis://redis:6379

# Supabase Configuration
SUPABASE_URL=https://YOUR_PROJECT.supabase.co
SUPABASE_SERVICE_KEY=YOUR_SUPABASE_SERVICE_KEY_HERE
SUPABASE_STORAGE_BUCKET=files

# OpenAI (for video planning)
OPENAI_API_KEY=sk-YOUR_OPENAI_KEY_HERE

# Gemini (for image generation)
GEMINI_API_KEY=YOUR_GEMINI_KEY_HERE
GEMINI_STYLE_REFERENCE_IMAGE=assets/style-reference/sample.jpeg

# Text-to-Speech: ElevenLabs (recommended)
ELEVENLABS_API_KEY=YOUR_ELEVENLABS_KEY_HERE
# Optional: ELEVENLABS_VOICE_ID=pNInz6obpgDQGcFmaJgB

# Alternative TTS: Cartesia (if not using ElevenLabs)
# CARTESIA_API_KEY=YOUR_CARTESIA_KEY_HERE
# CARTESIA_API_URL=https://api.cartesia.ai/v1
# CARTESIA_VOICE_ID=a0e99841-438c-4a64-b679-ae501e7d6091

# Optional: xAI Video Generation
# XAI_VIDEO_ENABLED=false
# XAI_API_KEY=YOUR_XAI_KEY_HERE

# Optional: Google Veo Video Generation
# VEO_ENABLED=false
# VEO_MODEL=veo-3.1-generate-preview

# Background Music
BACKGROUND_MUSIC_PATH=assets/music/music.mp3

# Worker Configuration
MAX_CONCURRENT_JOBS=3
```

**Important Notes:**
- Replace `YOUR_DB_PASSWORD_HERE` in the `DATABASE_URL` line with the same password you set in `DB_PASSWORD`
- Fill in ALL the API keys from your services
- Save and exit: `Ctrl+X`, then `Y`, then `Enter`

After saving, secure the file:
```bash
chmod 600 .env
```

## Step 3: Start Docker Services

Run the deployment script:

```bash
cd ~/apps/episod
chmod +x deploy.sh
./deploy.sh
```

This will:
- Check prerequisites
- Build Docker images
- Start all containers (PostgreSQL, Redis, API+Worker)
- Test the health endpoint

**If you see permission errors with Docker**, you may need to run with sudo:
```bash
sudo ./deploy.sh
```

Or add your user to the docker group (recommended):
```bash
sudo usermod -aG docker $USER
newgrp docker
# Now try again without sudo
./deploy.sh
```

## Step 4: Verify Services Are Running

Check if all containers are up:
```bash
docker compose -f docker-compose.prod.yml ps
```

You should see:
- `episod-db` (postgres) - Up
- `episod-redis` (redis) - Up
- `episod-api` (api) - Up

View logs to ensure no errors:
```bash
docker compose -f docker-compose.prod.yml logs -f
```

Press `Ctrl+C` to exit logs.

Test the API locally:
```bash
curl http://localhost:8080/health
# Should return: {"status":"ok"}
```

## Step 5: Set Up Nginx Reverse Proxy (For Public Access)

**Note:** You'll need a domain name pointing to your server's IP address for SSL.

### Install Nginx and Certbot

```bash
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

### Create Initial Nginx Configuration

First, let's set up a temporary HTTP configuration to get the SSL certificate:

```bash
# Replace YOUR_DOMAIN with your actual domain in the command below
export DOMAIN="yourdomain.com"

sudo tee /etc/nginx/sites-available/episod << EOF
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # Increase timeouts for video processing
        proxy_connect_timeout 300;
        proxy_send_timeout 300;
        proxy_read_timeout 300;
    }
}
EOF
```

### Enable the Site

```bash
# Remove default site
sudo rm -f /etc/nginx/sites-enabled/default

# Enable episod site
sudo ln -sf /etc/nginx/sites-available/episod /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload Nginx
sudo systemctl reload nginx
```

## Step 6: Obtain SSL Certificate

Create directory for certbot:
```bash
sudo mkdir -p /var/www/certbot
```

Get SSL certificate (replace `yourdomain.com` with your actual domain):
```bash
sudo certbot --nginx -d yourdomain.com
```

**Follow the prompts:**
1. Enter your email address
2. Agree to terms of service (Y)
3. Share email with EFF (optional - your choice)
4. Choose to redirect HTTP to HTTPS: **2** (recommended)

Certbot will automatically update your Nginx configuration with SSL.

## Step 7: Update Nginx Configuration with Full Settings

Now let's apply the full production configuration:

```bash
# Update the template with your domain
cd ~/apps/episod
sed "s/YOUR_DOMAIN/yourdomain.com/g" nginx.conf.template | sudo tee /etc/nginx/sites-available/episod

# Test configuration
sudo nginx -t

# Reload Nginx
sudo systemctl reload nginx
```

## Step 8: Configure Firewall (Recommended)

```bash
# Allow SSH (IMPORTANT - don't lock yourself out!)
sudo ufw allow 22/tcp

# Allow HTTP and HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable firewall
sudo ufw --force enable

# Check status
sudo ufw status
```

## Step 9: Test Your Deployment

Test the health endpoint via your domain:
```bash
curl https://yourdomain.com/health
# Should return: {"status":"ok"}
```

Create a test video project (replace YOUR_API_KEY with your BACKEND_API_KEY from .env):
```bash
curl -X POST https://yourdomain.com/v1/projects \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_API_KEY" \
  -d '{
    "topic": "The History of Coffee",
    "target_duration_seconds": 30
  }'
```

You'll get a response with a `project_id`. Check its status:
```bash
curl https://yourdomain.com/v1/projects/PROJECT_ID_HERE \
  -H "X-API-Key: YOUR_API_KEY"
```

## Useful Commands Reference

### Docker Management
```bash
# View all logs
docker compose -f docker-compose.prod.yml logs -f

# View specific service logs
docker compose -f docker-compose.prod.yml logs -f api
docker compose -f docker-compose.prod.yml logs -f postgres

# Check container status
docker compose -f docker-compose.prod.yml ps

# Restart all services
docker compose -f docker-compose.prod.yml restart

# Restart specific service
docker compose -f docker-compose.prod.yml restart api

# Stop all services
docker compose -f docker-compose.prod.yml down

# View resource usage
docker stats
```

### Update Deployment (After Code Changes)
```bash
cd ~/apps/episod
git pull origin main
docker compose -f docker-compose.prod.yml up -d --build
```

### Database Backup
```bash
# Create backup
docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U postgres faceless | gzip > ~/backup_$(date +%Y%m%d).sql.gz

# Restore backup
gunzip < ~/backup_20240101.sql.gz | docker compose -f docker-compose.prod.yml exec -T postgres psql -U postgres faceless
```

### View Nginx Logs
```bash
sudo tail -f /var/log/nginx/episod_access.log
sudo tail -f /var/log/nginx/episod_error.log
```

## Troubleshooting

### API not responding
```bash
# Check if containers are running
docker compose -f docker-compose.prod.yml ps

# Check API logs for errors
docker compose -f docker-compose.prod.yml logs api

# Test local connection
curl http://localhost:8080/health
```

### Database connection errors
```bash
# Check database logs
docker compose -f docker-compose.prod.yml logs postgres

# Test database connection
docker compose -f docker-compose.prod.yml exec postgres psql -U postgres -d faceless -c "SELECT 1;"
```

### Worker not processing jobs
```bash
# Check worker logs
docker compose -f docker-compose.prod.yml logs api | grep -i worker

# Check Redis
docker compose -f docker-compose.prod.yml exec redis redis-cli ping
```

### Permission denied errors
```bash
# Add your user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Or use sudo with docker commands
sudo docker compose -f docker-compose.prod.yml ps
```

### SSL certificate renewal
```bash
# Test renewal
sudo certbot renew --dry-run

# Force renewal (if needed)
sudo certbot renew --force-renewal
sudo systemctl reload nginx
```

## Security Checklist

- [ ] Strong `DB_PASSWORD` set in .env
- [ ] Secure `BACKEND_API_KEY` set in .env
- [ ] `.env` file permissions set to 600 (`chmod 600 .env`)
- [ ] Firewall enabled and configured
- [ ] SSL certificate installed
- [ ] CORS configured with your domain
- [ ] Regular backups scheduled

## Next Steps

1. Monitor your first video generation to ensure everything works
2. Set up automated database backups (see Database Backup section)
3. Monitor server resources with `htop` or `docker stats`
4. Check logs regularly for errors

## Need Help?

- Check the main README.md for API usage
- Review logs: `docker compose -f docker-compose.prod.yml logs -f`
- Test the API health endpoint: `curl http://localhost:8080/health`
- Verify environment variables are set correctly in `.env`
