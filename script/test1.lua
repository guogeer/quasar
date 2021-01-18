local testpkg = require("testpkg")

function testcall(p)
	p.I32 = 3032
	p.I = 3064
	p.I64 = 3164
	p.F32 = 3032.3200
	p.F64 = 3064.6400
	p.S = "hello test3"
	for i=1,#p.AI2 do
		p.AI2[i] = 10
	end
	for i=1,#p.AS2 do
		p.AS2[i] = "ss"
	end
	return 123,"Hello World"
end

function test_sum(m,n,p)
	local sum = m + n
	for i=1,#p do
		sum = sum + p[i]
	end
	return sum
end

function test_json()
	local t = {["A"]=1,["B"]=2,["S"]="hello world"}
	return t
end

function set_inherit(c)
	c.A1 = 10
end

function handle_map(m,a)
	m[10] = 20
	for k,v in m() do
		print(k,v)
	end

	local arr = a.Arr
	arr = arr + {} + {} + {["A1"]=3}
	for _,v in arr() do
		print(v.A1)
	end
	a.Arr = arr
end

function test_registry_overflow(a,n)
	local a2 = a:Add(n)
	return 1,2,3,4,5,a2
end
