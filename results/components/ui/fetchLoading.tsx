const FetchLoading = () => (
  <section className="flex flex-1 flex-col items-center justify-center">
    <h2 className="mb-4 text-center opacity-30">Fetching data...</h2>
    <span className="loader flex scale-150 items-center justify-center opacity-50">
      <p className="-translate-y-5 text-lg font-semibold opacity-50">. . .</p>
    </span>
  </section>
);

export default FetchLoading;
