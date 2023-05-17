/** @type {import('next').NextConfig} */
const nextConfig = {
  // output: "export",
  images: {
    domains: ["api-preprod.acroworldtour.com", "civlcomps.org"],
  },
  reactStrictMode: true,
};

module.exports = nextConfig;
