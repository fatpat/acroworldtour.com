import TextField from '@mui/material/TextField'

const InputMark = (props) => {
  let value = typeof(props.value) === "number" ? props.value * 10 : ""
  return (
    <TextField
      onChange={e => {
        const v = parseInt(e.target.value)
        if (!isNaN(v)) {
          v = Math.min(100, Math.max(0, v)) / 10
        }
        props.onChange(v)
      }}
      defaultValue={value}
      type="number"
      inputProps={{ tabIndex: props.tabindex ?? "0" }}
    />
  )
/*
  return (
    <NativeSelect onChange={e => {props.onChange(e)}}>
      <option value="">-</option>
      { Array.from({length: 21}, (v, k) => k*0.5).map(i => (<option value={i} selected={props.value==i}>{i}</option>))}
    </NativeSelect>
  )
*/
}

export default InputMark
