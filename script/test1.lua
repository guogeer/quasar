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
	return "123"
end

function test_sum(m,n)
	return m + n
end
