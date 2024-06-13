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
      'api.acroworldtour.com',
      'proxy.acroworldtour.com',
      'api.training.acroworldtour.com',
      'proxy.training.acroworldtour.com',
      'civlcomps.org',
      '172.17.0.1',
      'localhost',
      '127.0.0.1',
    ],
  },
  reactStrictMode: true,
};

module.exports = nextConfig;
