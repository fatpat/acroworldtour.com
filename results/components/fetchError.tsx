import classNames from "classnames";

const FetchError = () => (
  <section>
    <h2 className="mb-4 text-center">Couldn&apos;t fetch data.</h2>
    <article
      className={classNames(
        "flex w-full max-w-lg flex-col items-center justify-center rounded-xl bg-awtgrey-100 shadow"
      )}
    >
      <span className="loader translate-y-10"></span>
      <h2 className="-translate-y-20 rotate-90 text-xl font-semibold text-awtgrey-300">
        :(
      </h2>
    </article>{" "}
  </section>
);

export default FetchError;
