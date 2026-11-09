package api

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"familymenu/internal/store"
)

// Excel 导出：只用 Go 标准库 archive/zip 生成最小可用的 OOXML .xlsx，
// 不引入任何第三方库；Excel / WPS / Numbers 都能直接打开。
// 单元格用 inlineStr，中文不需要额外共享字符串表。

const excelContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const excelRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const excelWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="今日菜单" sheetId="1" r:id="rId1"/><sheet name="菜谱清单" sheetId="2" r:id="rId2"/></sheets>
</workbook>`

const excelWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const excelStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>
<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
<borders count="1"><border/></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs>
</styleSheet>`

// excelCol: 0->A, 25->Z, 26->AA
func excelCol(i int) string {
	s := ""
	for i >= 0 {
		s = string(rune('A'+i%26)) + s
		i = i/26 - 1
	}
	return s
}

func excelSheet(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		fmt.Fprintf(&b, `<row r="%d">`, ri+1)
		for ci, v := range row {
			fmt.Fprintf(&b, `<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">`, excelCol(ci), ri+1)
			_ = xml.EscapeText(&b, []byte(v))
			b.WriteString(`</t></is></c>`)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func (s *Server) exportExcel(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if !validDate(date) {
		jsonErr(w, http.StatusBadRequest, "日期格式应为 YYYY-MM-DD")
		return
	}

	plan := s.st.GetPlan(date)
	dishes := s.st.ListDishes()
	mealNames := map[string]string{}
	for _, m := range s.st.Meals() {
		mealNames[m.Key] = m.Label
	}
	memberNames := map[string]string{}
	for _, m := range s.st.ListMembers() {
		memberNames[m.ID] = m.Name
	}
	cuisineNames := map[string]string{}
	for _, c := range store.Cuisines {
		cuisineNames[c.Key] = c.Name
	}

	todayRows := [][]string{{"日期", "餐别", "菜名", "点菜人", "备注", "状态"}}
	for _, o := range plan.Orders {
		meal := mealNames[o.Meal]
		if meal == "" {
			meal = o.Meal
		}
		who := memberNames[o.UserID]
		if who == "" {
			who = o.UserName
		}
		status := "待做"
		if o.Done {
			status = "已上桌"
		}
		todayRows = append(todayRows, []string{date, meal, o.DishName, who, o.Note, status})
	}
	if len(todayRows) == 1 {
		todayRows = append(todayRows, []string{date, "", "今天还没点菜", "", "", ""})
	}

	spicyLabels := []string{"不辣", "微辣", "中辣", "特辣"}
	menuRows := [][]string{{"菜名", "菜系", "描述", "辣度", "耗时(分钟)", "标签", "配料", "做法步骤数", "收藏"}}
	for _, d := range dishes {
		cuisine := cuisineNames[d.Cuisine]
		if cuisine == "" {
			cuisine = d.Cuisine
		}
		sp := d.Spicy
		if sp < 0 {
			sp = 0
		}
		if sp > 3 {
			sp = 3
		}
		fav := ""
		if d.Favorite {
			fav = "⭐"
		}
		menuRows = append(menuRows, []string{
			d.Name, cuisine, d.Desc, spicyLabels[sp], fmt.Sprint(d.Minutes),
			strings.Join(d.Tags, "、"), strings.Join(d.Ingredients, "；"),
			fmt.Sprint(len(d.Steps)), fav,
		})
	}

	parts := []struct{ name, data string }{
		{"[Content_Types].xml", excelContentTypes},
		{"_rels/.rels", excelRootRels},
		{"xl/workbook.xml", excelWorkbook},
		{"xl/_rels/workbook.xml.rels", excelWorkbookRels},
		{"xl/styles.xml", excelStyles},
		{"xl/worksheets/sheet1.xml", excelSheet(todayRows)},
		{"xl/worksheets/sheet2.xml", excelSheet(menuRows)},
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="family-menu-%s.xlsx"`, strings.ReplaceAll(date, "-", "")))
	w.Header().Set("Cache-Control", "no-store")
	zw := zip.NewWriter(w)
	for _, p := range parts {
		entry, err := zw.Create(p.name)
		if err != nil {
			return
		}
		_, _ = entry.Write([]byte(p.data))
	}
	_ = zw.Close()
}
