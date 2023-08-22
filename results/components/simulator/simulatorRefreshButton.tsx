import IconButton from "@mui/material/IconButton";

import { SimulatorIcon } from "@/components/ui/icons";

interface Props {
  refresh: any;
}

const SimulatorRefreshButton = ({ refresh }: Props) => {
  return (
    <IconButton
      arai-label="simulate"
      onClick={(e) => {
        e.preventDefault();
        refresh();
      }}
    >
      <SimulatorIcon />
      <SimulatorIcon />
    </IconButton>
  );
};

export default SimulatorRefreshButton;
