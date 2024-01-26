<html>
<style>
td, th {
  text-align: left;
  padding: 0 8px;
}
</style>
<head><title>Index of {{ .Current }}</title></head>
<body bgcolor="white">
<h1>Index of {{ .Current }}</h1>
<hr>
<table>
  <tbody>
    <td><a href="{{ .Previous }}">../</td>
{{- range $dir := .Directories }}
    <tr>
      <td><a href="{{ $dir.Ref }}/">{{ $dir.Name }}/</a></td>
      <td>{{ $dir.MTime.Format "02-Jan-2006 15:04" }}</td>
      <td>-</td>
    </tr>
{{- end }}
{{- range $file := .Files }}
    <tr>
      <td><a href="{{ $file.Ref }}">{{ $file.Name }}</a></td>
      <td>{{ $file.MTime.Format "02-Jan-2006 15:04" }}</td>
      <td>{{ $file.Size }}</td>
    </tr>
{{- end }}
  </tbody>
</table>
<hr>
</body>
</html>