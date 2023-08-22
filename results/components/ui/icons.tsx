interface Props {
  className?: string;
}

/* Prefer icons from https://fontawesome.com/icons for consistency */

export const HomeIcon = ({ className }: Props) => (
  <svg
    width="76"
    height="76"
    viewBox="0 0 76 76"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M75.9999 34.3666V58.1254C75.9999 62.8592 74.1194 67.3991 70.7721 70.7464C67.4248 74.0937 62.8849 75.9742 58.1511 75.9742H48.2351V58.1254C48.2351 55.4955 47.1904 52.9733 45.3308 51.1137C43.4711 49.2541 40.949 48.2094 38.3191 48.2094C35.6892 48.2094 33.167 49.2541 31.3074 51.1137C29.4478 52.9733 28.403 55.4955 28.403 58.1254V75.9742H18.487C13.7532 75.9742 9.21331 74.0937 5.866 70.7464C2.5187 67.3991 0.638199 62.8592 0.638199 58.1254V34.3666C0.634722 31.7015 1.23117 29.0698 2.38333 26.6666C3.53549 24.2634 5.2138 22.1504 7.29383 20.4842L27.1259 4.54718C30.3061 2.01437 34.2515 0.635254 38.3171 0.635254C42.3827 0.635254 46.3281 2.01437 49.5083 4.54718L69.3403 20.4842C71.4215 22.1496 73.1009 24.2624 74.2538 26.6657C75.4067 29.069 76.0035 31.7011 75.9999 34.3666V34.3666Z" />
  </svg>
);

export const ResultsIcon = ({ className }: Props) => (
  <svg
    width="75"
    height="85"
    viewBox="0 0 75 85"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M65.625 9.375H56.25C56.2426 6.89088 55.2525 4.51062 53.4959 2.75407C51.7394 0.997522 49.3591 0.00741903 46.875 0L28.125 0C25.6409 0.00741903 23.2606 0.997522 21.5041 2.75407C19.7475 4.51062 18.7574 6.89088 18.75 9.375H9.375C6.89088 9.38242 4.51062 10.3725 2.75407 12.1291C0.997522 13.8856 0.00741903 16.2659 0 18.75V75C0.00741903 77.4841 0.997522 79.8644 2.75407 81.6209C4.51062 83.3775 6.89088 84.3676 9.375 84.375H65.625C68.1091 84.3676 70.4894 83.3775 72.2459 81.6209C74.0025 79.8644 74.9926 77.4841 75 75V18.75C74.9926 16.2659 74.0025 13.8856 72.2459 12.1291C70.4894 10.3725 68.1091 9.38242 65.625 9.375ZM14.0625 70.3125C13.1354 70.3125 12.2291 70.0376 11.4583 69.5225C10.6874 69.0074 10.0866 68.2754 9.73181 67.4188C9.37703 66.5623 9.2842 65.6198 9.46507 64.7105C9.64594 63.8012 10.0924 62.966 10.7479 62.3104C11.4035 61.6549 12.2387 61.2084 13.148 61.0276C14.0573 60.8467 14.9998 60.9395 15.8563 61.2943C16.7129 61.6491 17.4449 62.2499 17.96 63.0208C18.4751 63.7916 18.75 64.6979 18.75 65.625C18.75 66.8682 18.2561 68.0605 17.3771 68.9396C16.498 69.8186 15.3057 70.3125 14.0625 70.3125ZM14.0625 51.5625C13.1354 51.5625 12.2291 51.2876 11.4583 50.7725C10.6874 50.2574 10.0866 49.5254 9.73181 48.6688C9.37703 47.8123 9.2842 46.8698 9.46507 45.9605C9.64594 45.0512 10.0924 44.216 10.7479 43.5604C11.4035 42.9049 12.2387 42.4584 13.148 42.2776C14.0573 42.0967 14.9998 42.1895 15.8563 42.5443C16.7129 42.8991 17.4449 43.4999 17.96 44.2708C18.4751 45.0416 18.75 45.9479 18.75 46.875C18.75 48.1182 18.2561 49.3105 17.3771 50.1896C16.498 51.0686 15.3057 51.5625 14.0625 51.5625ZM14.0625 32.8125C13.1354 32.8125 12.2291 32.5376 11.4583 32.0225C10.6874 31.5074 10.0866 30.7754 9.73181 29.9188C9.37703 29.0623 9.2842 28.1198 9.46507 27.2105C9.64594 26.3012 10.0924 25.466 10.7479 24.8104C11.4035 24.1549 12.2387 23.7084 13.148 23.5276C14.0573 23.3467 14.9998 23.4395 15.8563 23.7943C16.7129 24.1491 17.4449 24.7499 17.96 25.5208C18.4751 26.2916 18.75 27.1979 18.75 28.125C18.75 29.3682 18.2561 30.5605 17.3771 31.4396C16.498 32.3186 15.3057 32.8125 14.0625 32.8125ZM60.9375 70.3125H32.8125C31.5693 70.3125 30.377 69.8186 29.4979 68.9396C28.6189 68.0605 28.125 66.8682 28.125 65.625C28.125 64.3818 28.6189 63.1895 29.4979 62.3104C30.377 61.4314 31.5693 60.9375 32.8125 60.9375H60.9375C62.1807 60.9375 63.373 61.4314 64.2521 62.3104C65.1311 63.1895 65.625 64.3818 65.625 65.625C65.625 66.8682 65.1311 68.0605 64.2521 68.9396C63.373 69.8186 62.1807 70.3125 60.9375 70.3125ZM60.9375 51.5625H32.8125C31.5693 51.5625 30.377 51.0686 29.4979 50.1896C28.6189 49.3105 28.125 48.1182 28.125 46.875C28.125 45.6318 28.6189 44.4395 29.4979 43.5604C30.377 42.6814 31.5693 42.1875 32.8125 42.1875H60.9375C62.1807 42.1875 63.373 42.6814 64.2521 43.5604C65.1311 44.4395 65.625 45.6318 65.625 46.875C65.625 48.1182 65.1311 49.3105 64.2521 50.1896C63.373 51.0686 62.1807 51.5625 60.9375 51.5625ZM60.9375 32.8125H32.8125C31.5693 32.8125 30.377 32.3186 29.4979 31.4396C28.6189 30.5605 28.125 29.3682 28.125 28.125C28.125 26.8818 28.6189 25.6895 29.4979 24.8104C30.377 23.9314 31.5693 23.4375 32.8125 23.4375H60.9375C62.1807 23.4375 63.373 23.9314 64.2521 24.8104C65.1311 25.6895 65.625 26.8818 65.625 28.125C65.625 29.3682 65.1311 30.5605 64.2521 31.4396C63.373 32.3186 62.1807 32.8125 60.9375 32.8125Z" />
  </svg>
);

export const SeasonsIcon = ({ className }: Props) => (
  <svg
    width="75"
    height="75"
    viewBox="0 0 448 512"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M128 0c17.7 0 32 14.3 32 32V64H288V32c0-17.7 14.3-32 32-32s32 14.3 32 32V64h48c26.5 0 48 21.5 48 48v48H0V112C0 85.5 21.5 64 48 64H96V32c0-17.7 14.3-32 32-32zM0 192H448V464c0 26.5-21.5 48-48 48H48c-26.5 0-48-21.5-48-48V192zm64 80v32c0 8.8 7.2 16 16 16h32c8.8 0 16-7.2 16-16V272c0-8.8-7.2-16-16-16H80c-8.8 0-16 7.2-16 16zm128 0v32c0 8.8 7.2 16 16 16h32c8.8 0 16-7.2 16-16V272c0-8.8-7.2-16-16-16H208c-8.8 0-16 7.2-16 16zm144-16c-8.8 0-16 7.2-16 16v32c0 8.8 7.2 16 16 16h32c8.8 0 16-7.2 16-16V272c0-8.8-7.2-16-16-16H336zM64 400v32c0 8.8 7.2 16 16 16h32c8.8 0 16-7.2 16-16V400c0-8.8-7.2-16-16-16H80c-8.8 0-16 7.2-16 16zm144-16c-8.8 0-16 7.2-16 16v32c0 8.8 7.2 16 16 16h32c8.8 0 16-7.2 16-16V400c0-8.8-7.2-16-16-16H208zm112 16v32c0 8.8 7.2 16 16 16h32c8.8 0 16-7.2 16-16V400c0-8.8-7.2-16-16-16H336c-8.8 0-16 7.2-16 16z" />
  </svg>
);

export const CompetitionsIcon = ({ className }: Props) => (
  <svg
    width="73"
    height="76"
    viewBox="0 0 73 76"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M14.8125 0H58.1721V6.92892H73V18.3077C73 26.4839 66.3482 33.1356 58.1721 33.1356H56.2474C53.4451 39.3101 47.8557 43.9293 41.0654 45.3921V54.6923H46.3776C52.6796 54.6923 54.0128 59.2705 53.8916 61.5596V75.479H19.093V61.5596C18.977 59.2705 20.3204 54.6923 26.6224 54.6923H31.9346V45.3921C25.1443 43.9447 19.5549 39.3101 16.7526 33.1356H14.8279C6.65176 33.1356 0 26.4839 0 18.3077V6.92892H14.8125V0Z" />
  </svg>
);

export const PilotsIcon = ({ className }: Props) => (
  <svg
    width="75"
    height="75"
    viewBox="0 0 75 75"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M54.1935 39.8673C57.437 36.5778 59.6352 32.4034 60.5119 27.8688C61.3885 23.3343 60.9044 18.6419 59.1205 14.3815C57.3366 10.1212 54.3324 6.48289 50.4855 3.92406C46.6387 1.36522 42.1209 0 37.5 0C32.8791 0 28.3613 1.36522 24.5145 3.92406C20.6676 6.48289 17.6634 10.1212 15.8795 14.3815C14.0956 18.6419 13.6115 23.3343 14.4881 27.8688C15.3648 32.4034 17.563 36.5778 20.8065 39.8673C14.684 42.2728 9.4264 46.4622 5.71614 51.8917C2.00588 57.3212 0.0143301 63.74 7.53527e-07 70.315C-0.000348423 70.9304 0.120659 71.5397 0.356104 72.1083C0.591549 72.6769 0.936815 73.1935 1.37216 73.6286C1.8075 74.0637 2.32439 74.4088 2.89326 74.6441C3.46213 74.8794 4.07183 75.0004 4.6875 75H70.3125C70.9282 75.0004 71.5379 74.8794 72.1067 74.6441C72.6756 74.4088 73.1925 74.0637 73.6278 73.6286C74.0632 73.1935 74.4084 72.6769 74.6439 72.1083C74.8793 71.5397 75.0003 70.9304 75 70.315C74.9857 63.74 72.9941 57.3212 69.2839 51.8917C65.5736 46.4622 60.316 42.2728 54.1935 39.8673Z" />
  </svg>
);

export const TeamsIcon = ({ className }: Props) => (
  <svg
    width="75"
    height="75"
    viewBox="0 0 640 512"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M64 64a64 64 0 1 1 128 0A64 64 0 1 1 64 64zM25.9 233.4C29.3 191.9 64 160 105.6 160h44.8c27 0 51 13.4 65.5 34.1c-2.7 1.9-5.2 4-7.5 6.3l-64 64c-21.9 21.9-21.9 57.3 0 79.2L192 391.2V464c0 26.5-21.5 48-48 48H112c-26.5 0-48-21.5-48-48V348.3c-26.5-9.5-44.7-35.8-42.2-65.6l4.1-49.3zM448 64a64 64 0 1 1 128 0A64 64 0 1 1 448 64zM431.6 200.4c-2.3-2.3-4.9-4.4-7.5-6.3c14.5-20.7 38.6-34.1 65.5-34.1h44.8c41.6 0 76.3 31.9 79.7 73.4l4.1 49.3c2.5 29.8-15.7 56.1-42.2 65.6V464c0 26.5-21.5 48-48 48H496c-26.5 0-48-21.5-48-48V391.2l47.6-47.6c21.9-21.9 21.9-57.3 0-79.2l-64-64zM272 240v32h96V240c0-9.7 5.8-18.5 14.8-22.2s19.3-1.7 26.2 5.2l64 64c9.4 9.4 9.4 24.6 0 33.9l-64 64c-6.9 6.9-17.2 8.9-26.2 5.2s-14.8-12.5-14.8-22.2V336H272v32c0 9.7-5.8 18.5-14.8 22.2s-19.3 1.7-26.2-5.2l-64-64c-9.4-9.4-9.4-24.6 0-33.9l64-64c6.9-6.9 17.2-8.9 26.2-5.2s14.8 12.5 14.8 22.2z" />
  </svg>
);

export const JudgesIcon = ({ className }: Props) => (
  <svg
    width="75"
    height="75"
    viewBox="0 0 512 512"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M318.6 9.4c-12.5-12.5-32.8-12.5-45.3 0l-120 120c-12.5 12.5-12.5 32.8 0 45.3l16 16c12.5 12.5 32.8 12.5 45.3 0l4-4L325.4 293.4l-4 4c-12.5 12.5-12.5 32.8 0 45.3l16 16c12.5 12.5 32.8 12.5 45.3 0l120-120c12.5-12.5 12.5-32.8 0-45.3l-16-16c-12.5-12.5-32.8-12.5-45.3 0l-4 4L330.6 74.6l4-4c12.5-12.5 12.5-32.8 0-45.3l-16-16zm-152 288c-12.5-12.5-32.8-12.5-45.3 0l-112 112c-12.5 12.5-12.5 32.8 0 45.3l48 48c12.5 12.5 32.8 12.5 45.3 0l112-112c12.5-12.5 12.5-32.8 0-45.3l-1.4-1.4L272 285.3 226.7 240 168 298.7l-1.4-1.4z" />
  </svg>
);

export const TricksIcon = ({ className }: Props) => (
  <svg
    width="75"
    height="75"
    viewBox="0 0 512 512"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M251.64 354.55c-1.4 0-88 119.9-88.7 119.9S76.34 414 76 413.25s86.6-125.7 86.6-127.4c0-2.2-129.6-44-137.6-47.1-1.3-.5 31.4-101.8 31.7-102.1.6-.7 144.4 47 145.5 47 .4 0 .9-.6 1-1.3.4-2 1-148.6 1.7-149.6.8-1.2 104.5-.7 105.1-.3 1.5 1 3.5 156.1 6.1 156.1 1.4 0 138.7-47 139.3-46.3.8.9 31.9 102.2 31.5 102.6-.9.9-140.2 47.1-140.6 48.8-.3 1.4 82.8 122.1 82.5 122.9s-85.5 63.5-86.3 63.5c-1-.2-89-125.5-90.9-125.5z" />
  </svg>
);

export const ChevronIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 512 512"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M233.4 406.6c12.5 12.5 32.8 12.5 45.3 0l192-192c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0L256 338.7 86.6 169.4c-12.5-12.5-32.8-12.5-45.3 0s-12.5 32.8 0 45.3l192 192z" />
  </svg>
);

export const FacebookIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 500 500"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M504 256C504 119 393 8 256 8S8 119 8 256c0 123.78 90.69 226.38 209.25 245V327.69h-63V256h63v-54.64c0-62.15 37-96.48 93.67-96.48 27.14 0 55.52 4.84 55.52 4.84v61h-31.28c-30.8 0-40.41 19.12-40.41 38.73V256h68.78l-11 71.69h-57.78V501C413.31 482.38 504 379.78 504 256z" />
  </svg>
);

export const InstagramIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 450 500"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M224.1 141c-63.6 0-114.9 51.3-114.9 114.9s51.3 114.9 114.9 114.9S339 319.5 339 255.9 287.7 141 224.1 141zm0 189.6c-41.1 0-74.7-33.5-74.7-74.7s33.5-74.7 74.7-74.7 74.7 33.5 74.7 74.7-33.6 74.7-74.7 74.7zm146.4-194.3c0 14.9-12 26.8-26.8 26.8-14.9 0-26.8-12-26.8-26.8s12-26.8 26.8-26.8 26.8 12 26.8 26.8zm76.1 27.2c-1.7-35.9-9.9-67.7-36.2-93.9-26.2-26.2-58-34.4-93.9-36.2-37-2.1-147.9-2.1-184.9 0-35.8 1.7-67.6 9.9-93.9 36.1s-34.4 58-36.2 93.9c-2.1 37-2.1 147.9 0 184.9 1.7 35.9 9.9 67.7 36.2 93.9s58 34.4 93.9 36.2c37 2.1 147.9 2.1 184.9 0 35.9-1.7 67.7-9.9 93.9-36.2 26.2-26.2 34.4-58 36.2-93.9 2.1-37 2.1-147.8 0-184.8zM398.8 388c-7.8 19.6-22.9 34.7-42.6 42.6-29.5 11.7-99.5 9-132.1 9s-102.7 2.6-132.1-9c-19.6-7.8-34.7-22.9-42.6-42.6-11.7-29.5-9-99.5-9-132.1s-2.6-102.7 9-132.1c7.8-19.6 22.9-34.7 42.6-42.6 29.5-11.7 99.5-9 132.1-9s102.7-2.6 132.1 9c19.6 7.8 34.7 22.9 42.6 42.6 11.7 29.5 9 99.5 9 132.1s2.7 102.7-9 132.1z" />
  </svg>
);

export const TikTokIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 448 512"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M448,209.91a210.06,210.06,0,0,1-122.77-39.25V349.38A162.55,162.55,0,1,1,185,188.31V278.2a74.62,74.62,0,1,0,52.23,71.18V0l88,0a121.18,121.18,0,0,0,1.86,22.17h0A122.18,122.18,0,0,0,381,102.39a121.43,121.43,0,0,0,67,20.14Z" />
  </svg>
);

export const TwitterIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 512 512"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M459.37 151.716c.325 4.548.325 9.097.325 13.645 0 138.72-105.583 298.558-298.558 298.558-59.452 0-114.68-17.219-161.137-47.106 8.447.974 16.568 1.299 25.34 1.299 49.055 0 94.213-16.568 130.274-44.832-46.132-.975-84.792-31.188-98.112-72.772 6.498.974 12.995 1.624 19.818 1.624 9.421 0 18.843-1.3 27.614-3.573-48.081-9.747-84.143-51.98-84.143-102.985v-1.299c13.969 7.797 30.214 12.67 47.431 13.319-28.264-18.843-46.781-51.005-46.781-87.391 0-19.492 5.197-37.36 14.294-52.954 51.655 63.675 129.3 105.258 216.365 109.807-1.624-7.797-2.599-15.918-2.599-24.04 0-57.828 46.782-104.934 104.934-104.934 30.213 0 57.502 12.67 76.67 33.137 23.715-4.548 46.456-13.32 66.599-25.34-7.798 24.366-24.366 44.833-46.132 57.827 21.117-2.273 41.584-8.122 60.426-16.243-14.292 20.791-32.161 39.308-52.628 54.253z" />
  </svg>
);

export const YouTubeIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 600 500"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M549.655 124.083c-6.281-23.65-24.787-42.276-48.284-48.597C458.781 64 288 64 288 64S117.22 64 74.629 75.486c-23.497 6.322-42.003 24.947-48.284 48.597-11.412 42.867-11.412 132.305-11.412 132.305s0 89.438 11.412 132.305c6.281 23.65 24.787 41.5 48.284 47.821C117.22 448 288 448 288 448s170.78 0 213.371-11.486c23.497-6.321 42.003-24.171 48.284-47.821 11.412-42.867 11.412-132.305 11.412-132.305s0-89.438-11.412-132.305zm-317.51 213.508V175.185l142.739 81.205-142.739 81.201z" />
  </svg>
);

export const WebsiteIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 512 512"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M0 256C0 209.4 12.47 165.6 34.27 127.1L144.1 318.3C166 357.5 207.9 384 256 384C270.3 384 283.1 381.7 296.8 377.4L220.5 509.6C95.9 492.3 0 385.3 0 256zM365.1 321.6C377.4 302.4 384 279.1 384 256C384 217.8 367.2 183.5 340.7 160H493.4C505.4 189.6 512 222.1 512 256C512 397.4 397.4 511.1 256 512L365.1 321.6zM477.8 128H256C193.1 128 142.3 172.1 130.5 230.7L54.19 98.47C101 38.53 174 0 256 0C350.8 0 433.5 51.48 477.8 128V128zM168 256C168 207.4 207.4 168 256 168C304.6 168 344 207.4 344 256C344 304.6 304.6 344 256 344C207.4 344 168 304.6 168 256z" />
  </svg>
);

export const WikipediaIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 640 512"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M640 51.2l-.3 12.2c-28.1.8-45 15.8-55.8 40.3-25 57.8-103.3 240-155.3 358.6H415l-81.9-193.1c-32.5 63.6-68.3 130-99.2 193.1-.3.3-15 0-15-.3C172 352.3 122.8 243.4 75.8 133.4 64.4 106.7 26.4 63.4.2 63.7c0-3.1-.3-10-.3-14.2h161.9v13.9c-19.2 1.1-52.8 13.3-43.3 34.2 21.9 49.7 103.6 240.3 125.6 288.6 15-29.7 57.8-109.2 75.3-142.8-13.9-28.3-58.6-133.9-72.8-160-9.7-17.8-36.1-19.4-55.8-19.7V49.8l142.5.3v13.1c-19.4.6-38.1 7.8-29.4 26.1 18.9 40 30.6 68.1 48.1 104.7 5.6-10.8 34.7-69.4 48.1-100.8 8.9-20.6-3.9-28.6-38.6-29.4.3-3.6 0-10.3.3-13.6 44.4-.3 111.1-.3 123.1-.6v13.6c-22.5.8-45.8 12.8-58.1 31.7l-59.2 122.8c6.4 16.1 63.3 142.8 69.2 156.7L559.2 91.8c-8.6-23.1-36.4-28.1-47.2-28.3V49.6l127.8 1.1.2.5z" />
  </svg>
);

export const WarningIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 512 512"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M256 32c14.2 0 27.3 7.5 34.5 19.8l216 368c7.3 12.4 7.3 27.7 .2 40.1S486.3 480 472 480H40c-14.3 0-27.6-7.7-34.7-20.1s-7-27.8 .2-40.1l216-368C228.7 39.5 241.8 32 256 32zm0 128c-13.3 0-24 10.7-24 24V296c0 13.3 10.7 24 24 24s24-10.7 24-24V184c0-13.3-10.7-24-24-24zm32 224a32 32 0 1 0 -64 0 32 32 0 1 0 64 0z" />
  </svg>
);

export const ThumbDownIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 512 512"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M323.8 477.2c-38.2 10.9-78.1-11.2-89-49.4l-5.7-20c-3.7-13-10.4-25-19.5-35l-51.3-56.4c-8.9-9.8-8.2-25 1.6-33.9s25-8.2 33.9 1.6l51.3 56.4c14.1 15.5 24.4 34 30.1 54.1l5.7 20c3.6 12.7 16.9 20.1 29.7 16.5s20.1-16.9 16.5-29.7l-5.7-20c-5.7-19.9-14.7-38.7-26.6-55.5c-5.2-7.3-5.8-16.9-1.7-24.9s12.3-13 21.3-13L448 288c8.8 0 16-7.2 16-16c0-6.8-4.3-12.7-10.4-15c-7.4-2.8-13-9-14.9-16.7s.1-15.8 5.3-21.7c2.5-2.8 4-6.5 4-10.6c0-7.8-5.6-14.3-13-15.7c-8.2-1.6-15.1-7.3-18-15.2s-1.6-16.7 3.6-23.3c2.1-2.7 3.4-6.1 3.4-9.9c0-6.7-4.2-12.6-10.2-14.9c-11.5-4.5-17.7-16.9-14.4-28.8c.4-1.3 .6-2.8 .6-4.3c0-8.8-7.2-16-16-16H286.5c-12.6 0-25 3.7-35.5 10.7l-61.7 41.1c-11 7.4-25.9 4.4-33.3-6.7s-4.4-25.9 6.7-33.3l61.7-41.1c18.4-12.3 40-18.8 62.1-18.8H384c34.7 0 62.9 27.6 64 62c14.6 11.7 24 29.7 24 50c0 4.5-.5 8.8-1.3 13c15.4 11.7 25.3 30.2 25.3 51c0 6.5-1 12.8-2.8 18.7C504.8 238.3 512 254.3 512 272c0 35.3-28.6 64-64 64l-92.3 0c4.7 10.4 8.7 21.2 11.8 32.2l5.7 20c10.9 38.2-11.2 78.1-49.4 89zM32 384c-17.7 0-32-14.3-32-32V128c0-17.7 14.3-32 32-32H96c17.7 0 32 14.3 32 32V352c0 17.7-14.3 32-32 32H32z" />
  </svg>
);

export const SimulatorIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    viewBox="0 0 640 512"
    fill="current"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
  >
    <path d="M274.9 34.3c-28.1-28.1-73.7-28.1-101.8 0L34.3 173.1c-28.1 28.1-28.1 73.7 0 101.8L173.1 413.7c28.1 28.1 73.7 28.1 101.8 0L413.7 274.9c28.1-28.1 28.1-73.7 0-101.8L274.9 34.3zM200 224a24 24 0 1 1 48 0 24 24 0 1 1 -48 0zM96 200a24 24 0 1 1 0 48 24 24 0 1 1 0-48zM224 376a24 24 0 1 1 0-48 24 24 0 1 1 0 48zM352 200a24 24 0 1 1 0 48 24 24 0 1 1 0-48zM224 120a24 24 0 1 1 0-48 24 24 0 1 1 0 48zm96 328c0 35.3 28.7 64 64 64H576c35.3 0 64-28.7 64-64V256c0-35.3-28.7-64-64-64H461.7c11.6 36 3.1 77-25.4 105.5L320 413.8V448zM480 328a24 24 0 1 1 0 48 24 24 0 1 1 0-48z" />
  </svg>
);

export const DeleteIcon = ({ className }: Props) => (
  <svg
    width="20"
    height="20"
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 448 512"
    fill="current"
    className={className}
  >
    <path d="M170.5 51.6L151.5 80h145l-19-28.4c-1.5-2.2-4-3.6-6.7-3.6H177.1c-2.7 0-5.2 1.3-6.7 3.6zm147-26.6L354.2 80H368h48 8c13.3 0 24 10.7 24 24s-10.7 24-24 24h-8V432c0 44.2-35.8 80-80 80H112c-44.2 0-80-35.8-80-80V128H24c-13.3 0-24-10.7-24-24S10.7 80 24 80h8H80 93.8l36.7-55.1C140.9 9.4 158.4 0 177.1 0h93.7c18.7 0 36.2 9.4 46.6 24.9zM80 128V432c0 17.7 14.3 32 32 32H336c17.7 0 32-14.3 32-32V128H80zm80 64V400c0 8.8-7.2 16-16 16s-16-7.2-16-16V192c0-8.8 7.2-16 16-16s16 7.2 16 16zm80 0V400c0 8.8-7.2 16-16 16s-16-7.2-16-16V192c0-8.8 7.2-16 16-16s16 7.2 16 16zm80 0V400c0 8.8-7.2 16-16 16s-16-7.2-16-16V192c0-8.8 7.2-16 16-16s16 7.2 16 16z" />
  </svg>
);
