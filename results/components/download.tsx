import classNames from "classnames";
import Link from "next/link";

import { SPORTING_CODE_MANUAL_URL } from "@/constants";

const Download = () => (
  <>
    <h2 className="mx-auto mt-auto">Downloads</h2>
    <section>
      <Link
        href={{
          pathname: SPORTING_CODE_MANUAL_URL,
        }}
        target="_blank"
        rel="noopener noreferrer"
        className={classNames(
          "mt-4 flex flex-col items-start rounded-xl bg-awt-dark-50 p-4 text-sm font-semibold text-orange-500 shadow",
          "hover:invert"
        )}
      >
        📑 FAI Sporting Code
      </Link>
    </section>
  </>
);

export default Download;
