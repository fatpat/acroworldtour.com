import Link from "next/link";

import { SPORTING_CODE_MANUAL_URL } from "@/config/flags";

const Download = () => (
  <article className="mt-8">
    <h2>Downloads</h2>
    <div className="rounded-xl bg-awtgrey-100 p-4 text-sm flex flex-col items-center">
      <Link
        href={{
          pathname: SPORTING_CODE_MANUAL_URL,
        }}
        target="_blank"
        className="rounded-md p-1 text-sm hover:bg-awtgrey-200 hover:text-orange-500 text-orange-400 font-semibold"
      >
        📑 FAI Sporting Code
      </Link>
    </div>
  </article>
);

export default Download;
