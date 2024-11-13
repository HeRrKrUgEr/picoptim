/** @type {import('tailwindcss').Config} */

const withMT = require("@material-tailwind/html/utils/withMT");
const colors = require("tailwindcss/colors");
module.exports = withMT({
  content: ["./templates/**/*.{html,js}"],
  theme: {
    colors: {
      sky: colors.sky,
    },
    extend: {},
  },
  plugins: [],
});
