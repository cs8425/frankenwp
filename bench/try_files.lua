local function ends_with(str, suffix)
	return #str >= #suffix and str:sub(-#suffix) == suffix
end

-- core.register_fetches("is_php", function(txn)
-- 	local path = txn.sf:path()

-- 	-- real file path
-- 	local fullpath = "/var/www/html" .. path
-- 	txn:Warning("[fullpath]" .. fullpath)

-- 	local ok, err = os.rename(fullpath, fullpath)
-- 	txn:Warning("[existsFile]" .. tostring(ok) .. "," .. tostring(err))
-- 	local php_ext = ends_with(path, ".php")
-- 	local slash = ends_with(path, "/")
-- 	if (ok and php_ext) or slash then
-- 		txn:Warning("[.php]" .. fullpath)

-- 		if (not ok) and slash then
-- 			txn.http:req_set_path("/index.php")
-- 			txn:Warning("[/index.php]" .. fullpath)
-- 		end
-- 		return true
-- 	end

-- 	return false
-- end)

core.register_action("try_files", {"http-req"}, function(txn)
	local path = txn.sf:path()

	-- real file path
	local fullpath = "/var/www/html" .. path
	-- txn:Warning("[fullpath]" .. fullpath)

	local ok, err = os.rename(fullpath, fullpath)
	-- txn:Warning("[existsFile]" .. tostring(ok) .. "," .. tostring(err))

	local php_ext = ends_with(path, ".php")
	if ok and php_ext then
		-- txn:Warning("[.php]" .. fullpath)
		txn:set_var('req.is_static', false)
		return
	end

	local slash = ends_with(path, "/")
	if ok and slash then
		-- try xxx/index.php
		local try_path = fullpath .. "index.php"
		local ok, err = os.rename(try_path, try_path)
		if ok then
			-- txn:Warning("[.php]" .. try_path)
			txn:set_var('req.is_static', false)
			return
		end
	end

	if ok then
		-- static
		-- txn:Warning("[static]" .. fullpath)
		txn:set_var('req.is_static', true)
		return
	end


	txn:set_var('req.is_static', false)
	-- txn.http:req_set_path("/index.php" .. path)
end)


