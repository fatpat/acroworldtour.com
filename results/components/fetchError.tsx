import classNames from "classnames";

const FetchError = () => (
  <article
    className={classNames(
      "flex h-32 w-full max-w-lg flex-col items-center justify-center rounded-xl bg-awtgrey-100 shadow"
    )}
  >
    <span className="loader translate-y-4"></span>
    <h2 className="-translate-y-10 text-lg font-semibold text-awtgrey-300">
      Fetching data...
    </h2>
  </article>
);

export default FetchError;
