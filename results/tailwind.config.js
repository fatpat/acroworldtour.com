/** @type {import('tailwindcss').Config} */

import colors from "tailwindcss/colors";

module.exports = {
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        awtgrey: colors.neutral,
      },
    },
  },
  plugins: [
    ({ addUtilities }) => {
      addUtilities({
        ".awt-background": {
          "@apply bg-cover bg-center bg-no-repeat": {},
        },
        ".awt-background-blend": {
          "@apply bg-white/50 bg-cover bg-center bg-no-repeat bg-blend-screen":
            {},
        },
      });
    },
    require("@tailwindcss/typography"),
  ],
};
