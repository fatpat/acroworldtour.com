/** @type {import('next').NextConfig} */

const nextConfig = {
  output: "export",
  images: {
    domains: ["api.acroworldtour.com", "civlcomps.org"],
    unoptimized: true,
  },
  reactStrictMode: true,
};

module.exports = nextConfig;
