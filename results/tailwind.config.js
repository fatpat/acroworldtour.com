/** @type {import('tailwindcss').Config} */

import colors from 'tailwindcss/colors';

module.exports = {
  content: [
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        'awt-dark': colors.slate,
        'awt-accent': colors.sky,
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms')({
      strategy: 'base',
    }),
    require('@tailwindcss/typography'),
  ],
};
