import classNames from "classnames";

const FetchLoading = () => (
  <section>
    <h2 className="mb-4 text-center opacity-50">Fetching events...</h2>
    <article
      className={classNames(
        "flex w-full max-w-lg flex-col items-center justify-center rounded-xl bg-awtgrey-100 shadow"
      )}
    >
      <span className="loader translate-y-10"></span>
      <h2 className="-translate-y-20
       text-lg font-semibold text-awtgrey-300">
        Fetching data...
      </h2>
    </article>{" "}
  </section>
);

export default FetchLoading;
