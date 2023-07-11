-- example reporting script which demonstrates a custom
-- done() function that prints results as JSON

done = function(summary, latency, requests)
   io.stderr:write("{\n")
   io.stderr:write(string.format("\t\"requests\": %d,\n", summary.requests))
   io.stderr:write(string.format("\t\"duration_us\": %0.2f,\n", summary.duration))
   io.stderr:write(string.format("\t\"bytes\": %d,\n", summary.bytes))
   io.stderr:write(string.format("\t\"rps\": %0.2f,\n", (summary.requests/summary.duration)*1e6))
   io.stderr:write(string.format("\t\"rps_stdev\": %0.2f,\n", requests.stdev))
   io.stderr:write(string.format("\t\"bytes_per_sec\": %0.2f,\n", (summary.bytes/summary.duration)*1e6))
   io.stderr:write(string.format("\t\"connect_errors\": %d,\n", summary.errors.connect))
   io.stderr:write(string.format("\t\"read_errors\": %d,\n", summary.errors.read))
   io.stderr:write(string.format("\t\"write_errors\": %d,\n", summary.errors.write))
   io.stderr:write(string.format("\t\"http_errors\": %d,\n", summary.errors.status))
   io.stderr:write(string.format("\t\"timeouts\": %d,\n", summary.errors.timeout))
   io.stderr:write(string.format("\t\"avg_lat_us\": %0.2f,\n", latency.mean))
   io.stderr:write(string.format("\t\"stdev_lat\": %0.2f,\n", latency.stdev))
   io.stderr:write(string.format("\t\"max_lat_us\": %0.2f,\n", latency.max))
   for _, p in pairs({90, 95, 99}) do
      n = latency:percentile(p)
      io.stderr:write(string.format("\t\"p%g_lat_us\": %d", p, n))
	  if p == 99 then
        io.stderr:write("\n")
  	  else
        io.stderr:write(",\n")
	  end
   end
   io.stderr:write("}\n")
   io.stderr:flush()
end

