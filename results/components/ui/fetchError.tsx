const FetchError = () => (
  <section className="flex flex-1 flex-col items-center justify-center">
    <h2 className="z-10 mb-4 text-center opacity-80">
      Couldn&apos;t fetch data.
    </h2>
    <span className="loader flex scale-150 items-center justify-center opacity-50">
      <p className="-translate-y-5 rotate-90 text-lg font-semibold opacity-80">
        :(
      </p>
    </span>
  </section>
);

export default FetchError;
