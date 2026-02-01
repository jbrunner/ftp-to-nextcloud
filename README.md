# FTP to NextCloud

FTP/FTPS gateway that proxies file operations to NextCloud via WebDAV.

WebDAV path: `${NEXTCLOUD_URL}/public.php/dav/files/${FTP_PASSWORD}`

## Setup in NextCloud

1. Create or select a directory to share
2. Click "Share" → "Create public link"
3. Adjust permissions if needed (view-only or allow editing)
4. Copy the share link (e.g., `https://nextcloud.example.com/s/Abc123XyZ`)
5. Extract the share token (`Abc123XyZ`) from the link

## Docker Usage

Use the NextCloud base URL (without the `/s/...` path):

```bash
docker pull ghcr.io/jbrunner/ftp-to-nextcloud:latest

docker run -d \
  -p 2121:2121 \
  -e NEXTCLOUD_URL=https://nextcloud.example.com \
  ghcr.io/jbrunner/ftp-to-nextcloud:latest
```

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `NEXTCLOUD_URL` | Yes | - | NextCloud base URL (from step 4 above, without share path) |
| `FTP_PORT` | No | `2121` | FTP server port |
| `PUBLIC_HOST` | No | - | Public IP for passive mode (required behind NAT/Kubernetes) |
| `PASV_MIN_PORT` | No | `30000` | Minimum port for passive mode |
| `PASV_MAX_PORT` | No | `30100` | Maximum port for passive mode |
| `FTP_TLS` | No | `false` | Enable FTPS with auto-generated self-signed certificate |
| `INSECURE_SKIP_VERIFY` | No | `false` | Skip TLS certificate verification for NextCloud connection |
| `DEBUG` | No | `false` | Enable debug logging |

## FTP Client Configuration

Use the share token from step 5 as the FTP password:

```
Host: your-server
Port: 2121
Username: ftp (username is ignored by server)
Password: Abc123XyZ (share token from setup step 5)
```
