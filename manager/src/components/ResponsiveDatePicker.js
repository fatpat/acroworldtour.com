import * as React from 'react';
import TextField from '@mui/material/TextField';
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns';
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider';
import { DatePicker } from '@mui/x-date-pickers/DatePicker';
import { MobileDatePicker } from '@mui/x-date-pickers/MobileDatePicker';
import { DesktopDatePicker } from '@mui/x-date-pickers/DesktopDatePicker';
import Stack from '@mui/material/Stack';

export default function ResponsiveDatePicker(props) {
  const [value, setValue] = React.useState(props.default);

  return (
    <LocalizationProvider dateAdapter={AdapterDateFns}>
      <DatePicker
        views={props.views ?? ['year', 'day']}
        label={props.label}
        value={value}
        onChange={(newValue) => {
          newValue.toJSON = function() {
            return `${String(this.getFullYear())}-${String(this.getMonth() + 1).padStart(2, "0")}-${String(this.getDate()).padStart(2, "0")}`
          }
          setValue(newValue);
          if (props.onChange) props.onChange(newValue)
        }}
        renderInput={(params) => <TextField {...params} />}
      />
    </LocalizationProvider>
  );
}
