/** @type {import('next').NextConfig} */

const nextConfig = {
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: [
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff',
          },
        ],
      },
    ];
  },
  images: {
    domains: [
      '**.acroworldtour.com',
      'civlcomps.org',
      '172.17.0.1',
      'localhost',
      '127.0.0.1',
    ],
  },
  reactStrictMode: true,
};

module.exports = nextConfig;
