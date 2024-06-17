const path = require('path')

const nextConfig = {
  output: 'export',

  trailingSlash: false,
  reactStrictMode: false,
  images: {
    loader: "custom",
    unoptimized: true
  },
  experimental: {
    esmExternals: false,
  },
  webpack: config => {
    config.resolve.alias = {
      ...config.resolve.alias,
      apexcharts: path.resolve(__dirname, './node_modules/apexcharts-clevision')
    }

    return config
  }
}

module.exports = nextConfig
