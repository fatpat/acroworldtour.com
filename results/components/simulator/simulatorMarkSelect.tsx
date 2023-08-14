interface Props {
  value: number | undefined;
  defaultValue: number;
  setValue: any;
  className: string;
}

const SimulatorMarkSelect = ({
  value,
  defaultValue,
  setValue,
  className,
}: Props) => {
  return (
    <select
      value={value || defaultValue}
      onChange={(e) => setValue(parseFloat(e.target.value))}
      className={className}
    >
      {[...Array(21).keys()].map((mark) => {
        mark = mark / 2;
        return (
          <option key={`name${mark}`} value={mark}>
            {mark}
          </option>
        );
      })}
    </select>
  );
};

export default SimulatorMarkSelect;
