-- example reporting script which demonstrates a custom
-- done() function that prints results as JSON

done = function(summary, latency, requests)
   file = io.open("output.json", "w+")
   io.output(file)
   io.write("{\n")
   io.write(string.format("\t\"requests\": %d,\n", summary.requests))
   io.write(string.format("\t\"duration_in_microseconds\": %0.2f,\n", summary.duration))
   io.write(string.format("\t\"bytes\": %d,\n", summary.bytes))
   io.write(string.format("\t\"rps\": %0.2f,\n", (summary.requests/summary.duration)*1e6))
   io.write(string.format("\t\"rps_stdev\": %0.2f,\n", requests.stdev))
   io.write(string.format("\t\"bytes_per_sec\": %0.2f,\n", (summary.bytes/summary.duration)*1e6))
   io.write(string.format("\t\"connect_errors\": %d,\n", summary.errors.connect))
   io.write(string.format("\t\"read_errors\": %d,\n", summary.errors.read))
   io.write(string.format("\t\"write_errors\": %d,\n", summary.errors.write))
   io.write(string.format("\t\"http_errors\": %d,\n", summary.errors.status))
   io.write(string.format("\t\"timeouts\": %d,\n", summary.errors.timeout))
   io.write(string.format("\t\"avg_lat\": %0.2f,\n", latency.mean))
   io.write(string.format("\t\"stdev_lat\": %0.2f,\n", latency.stdev))
   io.write(string.format("\t\"max_lat\": %0.2f,\n", latency.max))
   io.write("\t\"lat_distribution\": [\n")
   for _, p in pairs({ 50, 75, 90, 99}) do
      io.write("\t\t{\n")
      n = latency:percentile(p)
      io.write(string.format("\t\t\t\"percentile\": %g,\n\t\t\t\"lat_in_microseconds\": %d\n", p, n))
      if p == 99 then 
          io.write("\t\t}\n")
      else 
          io.write("\t\t},\n")
      end
   end
   io.write("\t]\n}\n")
end

