package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/umahmood/haversine"
)

// Направления компаса
const (
	DirectionNorth     = "Север"
	DirectionNorthEast = "Северо-восток"
	DirectionEast      = "Восток"
	DirectionSouthEast = "Юго-восток"
	DirectionSouth     = "Юг"
	DirectionSouthWest = "Юго-запад"
	DirectionWest      = "Запад"
	DirectionNorthWest = "Северо-запад"
)

// Расчет расстояния между двумя точками в километрах
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	_, km := haversine.Distance(haversine.Coord{Lat: lat1, Lon: lon1}, haversine.Coord{Lat: lat2, Lon: lon2})
	return km
}

// Расчет расстояния в метрах
func calculateDistanceMeters(lat1, lon1, lat2, lon2 float64) int {
	km := calculateDistance(lat1, lon1, lat2, lon2)
	return int(km * 1000)
}

// Вычисление направления от текущей позиции к цели
func calculateDirection(fromLat, fromLon, toLat, toLon float64) string {
	// Конвертируем градусы в радианы
	fromLatRad := fromLat * math.Pi / 180
	fromLonRad := fromLon * math.Pi / 180
	toLatRad := toLat * math.Pi / 180
	toLonRad := toLon * math.Pi / 180

	// Вычисляем разность долгот
	deltaLon := toLonRad - fromLonRad

	// Вычисляем азимут
	y := math.Sin(deltaLon) * math.Cos(toLatRad)
	x := math.Cos(fromLatRad)*math.Sin(toLatRad) - math.Sin(fromLatRad)*math.Cos(toLatRad)*math.Cos(deltaLon)

	bearing := math.Atan2(y, x)

	// Конвертируем в градусы
	bearingDegrees := bearing * 180 / math.Pi

	// Нормализуем к 0-360 градусам
	if bearingDegrees < 0 {
		bearingDegrees += 360
	}

	// Определяем направление по компасу
	return degreesToDirection(bearingDegrees)
}

// Конвертация градусов в направление компаса
func degreesToDirection(degrees float64) string {
	directions := []string{
		DirectionNorth,     // 0°
		DirectionNorthEast, // 45°
		DirectionEast,      // 90°
		DirectionSouthEast, // 135°
		DirectionSouth,     // 180°
		DirectionSouthWest, // 225°
		DirectionWest,      // 270°
		DirectionNorthWest, // 315°
	}

	// Добавляем 22.5 для корректного округления
	index := int((degrees+22.5)/45) % 8
	return directions[index]
}

// Получение ASCII компаса с выделенным направлением
func getCompass(direction string) string {
	compasses := map[string]string{
		DirectionNorth: `
⠀⠀⠀🔴⠀⠀⠀
⠀⠀⠀⠀↑⠀⠀⠀
⚪ ← ⚫ → ⚪
⠀⠀⠀⠀↓⠀⠀⠀
⠀⠀⠀⠀⚪⠀⠀⠀`,
		DirectionNorthEast: `
⠀⠀⠀⚪⠀🔴⠀
⠀⠀⠀⠀↑⠀↗⠀
⚪ ← ⚫ → ⚪
⠀⠀⠀⠀↓⠀⠀⠀
⠀⠀⠀⠀⚪⠀⠀⠀`,
		DirectionEast: `
⠀⠀⠀⠀⚪⠀⠀⠀
⠀⠀⠀⠀↑⠀⠀⠀
⚪ ← ⚫ → 🔴
⠀⠀⠀⠀↓⠀⠀⠀
⠀⠀⠀⠀⚪⠀⠀⠀`,
		DirectionSouthEast: `
⠀⠀⠀⠀⚪⠀⠀⠀
⠀⠀⠀⠀↑⠀⠀⠀
⚪ ← ⚫ → ⚪
⠀⠀⠀⠀↓⠀↘⠀
⠀⠀⠀⠀⚪⠀🔴⠀`,
		DirectionSouth: `
⠀⠀⠀⠀⚪⠀⠀⠀
⠀⠀⠀⠀↑⠀⠀⠀
⚪ ← ⚫ → ⚪
⠀⠀⠀⠀↓⠀⠀⠀
⠀⠀⠀⠀🔴⠀⠀⠀`,
		DirectionSouthWest: `
⠀⠀⠀⠀⚪⠀⠀⠀
⠀⠀⠀⠀↑⠀⠀⠀
⚪ ← ⚫ → ⚪
⠀⠀↙⠀↓⠀⠀⠀
🔴⠀⠀⚪⠀⠀⠀`,
		DirectionWest: `
⠀⠀⠀⠀⚪⠀⠀⠀
⠀⠀⠀⠀↑⠀⠀⠀
🔴 ← ⚫ → ⚪
⠀⠀⠀⠀↓⠀⠀⠀
⠀⠀⠀⠀⚪⠀⠀⠀`,
		DirectionNorthWest: `
🔴⠀⠀⚪⠀⠀⠀
⠀↖⠀⠀↑⠀⠀⠀
⚪ ← ⚫ → ⚪
⠀⠀⠀⠀↓⠀⠀⠀
⠀⠀⠀⠀⚪⠀⠀⠀`,
	}

	if compass, exists := compasses[direction]; exists {
		return compass
	}
	return `
⠀⠀⠀📍⠀⠀⠀
⠀⠀⠀⠀↑⠀⠀⠀
⚪ ← ⚫ → ⚪
⠀⠀⠀⠀↓⠀⠀⠀
⠀⠀⠀⠀⚪⠀⠀⠀`
}

// Получение стрелки для указания направления (увеличенные стрелки)
func getDirectionArrow(direction string) string {
	arrows := map[string]string{
		DirectionNorth:     "⬆️⬆️",
		DirectionNorthEast: "↗️↗️",
		DirectionEast:      "➡️➡️",
		DirectionSouthEast: "↘️↘️",
		DirectionSouth:     "⬇️⬇️",
		DirectionSouthWest: "↙️↙️",
		DirectionWest:      "⬅️⬅️",
		DirectionNorthWest: "↖️↖️",
	}

	if arrow, exists := arrows[direction]; exists {
		return arrow
	}
	return "📍📍"
}

// Форматирование сообщения с направлением и расстоянием
func formatDirectionMessage(fromLat, fromLon, toLat, toLon float64) string {
	distance := calculateDistanceMeters(fromLat, fromLon, toLat, toLon)
	direction := calculateDirection(fromLat, fromLon, toLat, toLon)
	arrow := getDirectionArrow(direction)
	compass := getCompass(direction)

	var distanceText string
	if distance >= 1000 {
		distanceText = fmt.Sprintf("*%.1f км*", float64(distance)/1000)
	} else {
		distanceText = fmt.Sprintf("*%d м*", distance)
	}

	// Создаем красивое форматированное сообщение с компасом
	message := fmt.Sprintf(` ═══ НАВИГАЦИЯ ═══
%s

   %s *%s* %s

📏 Расстояние: %s

═══════════════════`,
		compass, arrow, strings.ToUpper(direction), arrow, distanceText)

	return message
}

// Проверка, достиг ли пользователь цели
func isTargetReached(fromLat, fromLon, toLat, toLon float64, targetDistance float64) bool {
	distance := calculateDistance(fromLat, fromLon, toLat, toLon) * 1000 // в метрах
	return distance <= targetDistance
}
