# Quick Start Deployment Guide

This is a condensed version for quick deployment. See `DEPLOYMENT.md` for detailed instructions.

## Prerequisites Checklist

- [ ] Hetzner server running Ubuntu/Debian
- [ ] Docker and Docker Compose installed
- [ ] FFmpeg installed on server
- [ ] Domain name pointing to server IP
- [ ] Ports 80 and 443 open
- [ ] All API keys ready (OpenAI, Gemini, Supabase, TTS provider)

## Quick Deployment Steps

### 1. Push Code to Git (Local Machine)

```bash
git add .
git commit -m "Deployment ready"
git push origin main
```

### 2. Connect to Server

```bash
ssh root@YOUR_SERVER_IP
```

### 3. Install Nginx and Certbot (if not installed)

```bash
apt update && apt upgrade -y
apt install -y nginx certbot python3-certbot-nginx git
```

### 4. Clone Repository

```bash
cd /opt
git clone YOUR_REPOSITORY_URL faceless
cd faceless
```

### 5. Configure Environment

```bash
nano .env
```

**Minimum required variables:**
```bash
# Security
DB_PASSWORD=your_strong_password
BACKEND_API_KEY=your_secure_api_key

# CORS
CORS_ALLOWED_ORIGINS=https://yourdomain.com

# Supabase
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_SERVICE_KEY=your_service_key
SUPABASE_STORAGE_BUCKET=faceless-videos

# AI Services
OPENAI_API_KEY=sk-xxx
GEMINI_API_KEY=xxx
ELEVENLABS_API_KEY=xxx

# Worker
WORKER_ENABLED=true
MAX_CONCURRENT_JOBS=3
```

Save and secure:
```bash
chmod 600 .env
```

### 6. Deploy with Script

```bash
./deploy.sh
```

This will build and start all containers. Wait for it to complete.

### 7. Configure Nginx

Create temporary HTTP config for SSL certificate:

```bash
cat > /etc/nginx/sites-available/faceless-temp << 'EOF'
server {
    listen 80;
    server_name yourdomain.com;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
EOF

# Replace yourdomain.com with your actual domain
sed -i 's/yourdomain.com/YOUR_DOMAIN/g' /etc/nginx/sites-available/faceless-temp

ln -sf /etc/nginx/sites-available/faceless-temp /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

### 8. Get SSL Certificate

```bash
mkdir -p /var/www/certbot
certbot --nginx -d yourdomain.com
```

Follow prompts, agree to terms, choose redirect option (recommended: Yes).

### 9. Switch to Full Nginx Config

```bash
# Update the template with your domain
sed -i 's/YOUR_DOMAIN/yourdomain.com/g' /opt/faceless/nginx.conf.template
cp /opt/faceless/nginx.conf.template /etc/nginx/sites-available/faceless

# Enable full config
rm /etc/nginx/sites-enabled/faceless-temp
ln -sf /etc/nginx/sites-available/faceless /etc/nginx/sites-enabled/

nginx -t && systemctl reload nginx
```

### 10. Test Deployment

```bash
# Health check
curl https://yourdomain.com/health

# Create test project
curl -X POST https://yourdomain.com/v1/projects \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_API_KEY" \
  -d '{"topic": "The History of Coffee", "target_duration_seconds": 30}'
```

## Common Commands

```bash
# View logs
docker compose -f docker-compose.prod.yml logs -f

# Restart services
docker compose -f docker-compose.prod.yml restart

# Update after git pull
git pull
docker compose -f docker-compose.prod.yml up -d --build

# Stop everything
docker compose -f docker-compose.prod.yml down
```

## Firewall Setup (Recommended)

```bash
apt install ufw
ufw allow 22/tcp   # SSH
ufw allow 80/tcp   # HTTP
ufw allow 443/tcp  # HTTPS
ufw enable
```

## Troubleshooting

**API not responding:**
```bash
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs api
curl http://localhost:8080/health
```

**Database issues:**
```bash
docker compose -f docker-compose.prod.yml logs postgres
docker compose -f docker-compose.prod.yml exec postgres pg_isready -U postgres
```

**Worker not processing:**
```bash
docker compose -f docker-compose.prod.yml logs api | grep -i worker
docker compose -f docker-compose.prod.yml exec redis redis-cli ping
```

## Need Help?

See full documentation in `DEPLOYMENT.md` or check the logs for specific error messages.
