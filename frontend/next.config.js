/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'standalone',
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://backend:8080/api/:path*',
      },
      // Note: WebSocket (/ws) must be proxied at the Nginx/reverse-proxy level.
      // Next.js rewrites() does not support WebSocket upgrades.
    ]
  },
}

module.exports = nextConfig
