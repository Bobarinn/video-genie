# Security Hardening Guide

This guide will help you secure your episod deployment on your Hetzner server.

## ⚠️ Current Security Issues to Fix

If you deployed before this guide existed, you may have these issues:

1. **PostgreSQL exposed publicly** (port 5433)
2. **Redis exposed publicly** (port 6379)
3. **API port 8080 accessible without reverse proxy**

## ✅ Secure Configuration (What We Want)

**Docker Services:**
- PostgreSQL → Internal only (no public port)
- Redis → Internal only (no public port)
- API → Localhost only (127.0.0.1:8080)
- Caddy → Public entrypoint (ports 80/443)

**Firewall (UFW):**
```
22/tcp    ALLOW    (SSH)
80/tcp    ALLOW    (HTTP)
443/tcp   ALLOW    (HTTPS)
```

## 🔧 Fix Security Issues on Your Server

### Step 1: Check Current Status

```bash
# Check what ports are publicly exposed
sudo ufw status

# Check Docker port bindings
docker compose -f docker-compose.prod.yml ps
```

### Step 2: Pull Latest Secure Configuration

```bash
cd ~/apps/episod
git pull origin main
```

### Step 3: Restart Services with Secure Config

```bash
# Stop and remove current containers
docker compose -f docker-compose.prod.yml down

# Start with updated secure configuration
docker compose -f docker-compose.prod.yml up -d --build

# Verify containers are running
docker compose -f docker-compose.prod.yml ps
```

### Step 4: Verify Internal-Only Access

**Test that database is NOT publicly accessible:**
```bash
# This should timeout/fail (good!)
timeout 5 nc -zv YOUR_SERVER_IP 5433 || echo "✅ Port 5433 is NOT exposed (secure)"

# This should timeout/fail (good!)
timeout 5 nc -zv YOUR_SERVER_IP 6379 || echo "✅ Port 6379 is NOT exposed (secure)"
```

**Test that services work internally:**
```bash
# Inside the Docker network, they should work
docker compose -f docker-compose.prod.yml exec api nc -zv postgres 5432
docker compose -f docker-compose.prod.yml exec api nc -zv redis 6379

# Should both return "open" or "succeeded"
```

### Step 5: Close Unnecessary Firewall Ports

```bash
# Check current firewall status
sudo ufw status numbered

# If port 8080 is open, close it
sudo ufw delete allow 8080/tcp

# If postgres port is open, close it
sudo ufw delete allow 5433/tcp

# If redis port is open, close it
sudo ufw delete allow 6379/tcp

# Your firewall should only allow these:
sudo ufw status
# Should show:
# 22/tcp    ALLOW
# 80/tcp    ALLOW
# 443/tcp   ALLOW
```

### Step 6: Verify Everything Still Works

```bash
# Test via Caddy (should work)
curl https://video.xophie.ai/health
# Should return: {"status":"ok"}

# Test direct API access from server (should work)
curl http://localhost:8080/health
# Should return: {"status":"ok"}

# Test direct API from outside (should FAIL - this is good!)
# From your local machine, not the server:
curl http://YOUR_SERVER_IP:8080/health
# Should timeout or connection refused (secure!)
```

## 🔒 Additional Security Recommendations

### 1. Add Redis Password (Optional but Recommended)

Edit your `.env`:
```bash
nano ~/apps/episod/.env
```

Add:
```bash
REDIS_PASSWORD=your_secure_redis_password_here
```

Update `docker-compose.prod.yml` to use password:
```yaml
redis:
  image: redis:7-alpine
  command: redis-server --requirepass ${REDIS_PASSWORD}
  # ... rest of config
```

Update Redis URL in API config:
```bash
REDIS_URL=redis://:${REDIS_PASSWORD}@redis:6379
```

### 2. Regular Security Updates

```bash
# Update system packages
sudo apt update && sudo apt upgrade -y

# Update Docker images periodically
cd ~/apps/episod
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### 3. Enable Automated Security Updates

```bash
sudo apt install unattended-upgrades
sudo dpkg-reconfigure --priority=low unattended-upgrades
```

### 4. Monitor Failed Login Attempts

```bash
# Install fail2ban
sudo apt install fail2ban

# It will automatically ban IPs with too many failed SSH attempts
```

### 5. Database Backups

Create automated backups:
```bash
# Create backup script
cat > ~/backup-database.sh << 'EOF'
#!/bin/bash
BACKUP_DIR=~/backups
mkdir -p $BACKUP_DIR
DATE=$(date +%Y%m%d_%H%M%S)
cd ~/apps/episod
docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U postgres faceless | gzip > $BACKUP_DIR/backup_$DATE.sql.gz
# Keep only last 7 backups
ls -t $BACKUP_DIR/backup_*.sql.gz | tail -n +8 | xargs -r rm
echo "✅ Backup completed: backup_$DATE.sql.gz"
EOF

chmod +x ~/backup-database.sh

# Test it
~/backup-database.sh

# Schedule daily backups at 2 AM
(crontab -l 2>/dev/null; echo "0 2 * * * ~/backup-database.sh") | crontab -
```

### 6. Set Up SSH Key Authentication (Disable Password Auth)

If not already done:
```bash
# On your local machine, create SSH key if you don't have one
ssh-keygen -t ed25519

# Copy to server
ssh-copy-id deploy@YOUR_SERVER_IP

# Test key-based login
ssh deploy@YOUR_SERVER_IP

# Once confirmed working, disable password authentication
sudo nano /etc/ssh/sshd_config
# Set: PasswordAuthentication no
# Save and restart SSH
sudo systemctl restart sshd
```

### 7. Monitor Disk Space

```bash
# Check disk usage
df -h

# Set up monitoring alert (basic)
cat > ~/check-disk.sh << 'EOF'
#!/bin/bash
USAGE=$(df -h / | awk 'NR==2 {print $5}' | sed 's/%//')
if [ $USAGE -gt 80 ]; then
    echo "⚠️  WARNING: Disk usage is at ${USAGE}%"
fi
EOF

chmod +x ~/check-disk.sh

# Run daily
(crontab -l 2>/dev/null; echo "0 9 * * * ~/check-disk.sh") | crontab -
```

## 🎯 Security Checklist

After following this guide, verify:

- [ ] PostgreSQL not accessible from internet
- [ ] Redis not accessible from internet
- [ ] API only accessible via Caddy (https://video.xophie.ai)
- [ ] Direct API port 8080 blocked in firewall
- [ ] UFW only allows ports 22, 80, 443
- [ ] SSL certificate active (via Caddy)
- [ ] Strong passwords in `.env` file
- [ ] `.env` file permissions set to 600
- [ ] Database backups scheduled
- [ ] System updates enabled

## 🆘 Troubleshooting

**"I can't access my API anymore"**
- Make sure Caddy is running: `sudo systemctl status caddy`
- Check Caddy config: `sudo caddy validate --config /etc/caddy/Caddyfile`
- Verify containers are up: `docker compose -f docker-compose.prod.yml ps`

**"Database connection errors after securing"**
- Containers talk via Docker network, not exposed ports
- Check DATABASE_URL uses `postgres:5432` (not localhost:5433)
- Restart containers: `docker compose -f docker-compose.prod.yml restart`

**"I need to access the database from my local machine"**
Use SSH tunnel:
```bash
# From your local machine
ssh -L 5432:localhost:5432 deploy@YOUR_SERVER_IP

# In another terminal, connect to localhost:5432
psql -h localhost -p 5432 -U postgres -d faceless
```

## 📚 Additional Resources

- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Caddy Security](https://caddyserver.com/docs/security)
- [Ubuntu Server Security](https://ubuntu.com/server/docs/security-introduction)

---

Last updated: 2026-02-09
