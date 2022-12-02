/*
* jmu, nov 2022
*
* this function is invoked upon the existence of an event in object storage bucket
* payload contains info about the file that triggered the event
* the content of the file is read in a buffer and is passed to the vision engine
* if objects detected belongs to a list of names set as a parameter in hte function configuration,
* then a notification is sent to a ons topic
*
*/
package main
//
import (
	"context"
	"fmt"
	aivision65 "github.com/oracle/oci-go-sdk/v65/aivision"		// https://pkg.go.dev/github.com/oracle/oci-go-sdk/v65/aivision
	common65 "github.com/oracle/oci-go-sdk/v65/common"
	auth65 "github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/ons"						// https://pkg.go.dev/github.com/oracle/oci-go-sdk/v65
	"github.com/oracle/oci-go-sdk/example/helpers"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/common/auth"
	"github.com/oracle/oci-go-sdk/objectstorage"				// https://pkg.go.dev/github.com/oracle/oci-go-sdk/v65/objectstorage
	fdk "github.com/fnproject/fdk-go"        	 				// https://pkg.go.dev/github.com/fnproject/fdk-go@v0.0.13
	"io"
	"io/ioutil"
	"log"
	"strings"
	"strconv"
	"encoding/json"
	"github.com/google/uuid"
)
//
const DEBUG = 0 // extra log when set to 1
//
type tAdditionalDetails struct {
	BucketName      	string `json:"bucketName"`
	Namespace 			string `json:"namespace"`
}
type tData struct {
	CompartmentId      	string `json:"compartmentId"`
	ResourceName 		string `json:"resourceName"`
	ResourceId    		string `json:"resourceId"`
	AdditionalDetails	tAdditionalDetails `json:"additionalDetails"`
}
//
type tEvent struct {
	EventType		   string `json:eventType`
	Data               tData `json:"data"`
}
/*****************************************************************************
	main
******************************************************************************/
func main() {
	fdk.Handle(fdk.HandlerFunc(handler))
}
/*****************************************************************************
	handler function
******************************************************************************/
func handler(ctx context.Context, in io.Reader, out io.Writer) {
	//
	// context
	traceIt(ctx)
	newCtx := fdk.GetContext(ctx)
	traceIt(newCtx)
	//
	/*
		gather environment variables
	*/
	objects_to_search_for := strings.Split(strings.ToUpper(newCtx.Config()["SEARCH_FOR"]), ",")
	confidence, _ := strconv.ParseFloat(newCtx.Config()["CONFIDENCE"], 32)
	mytopic := newCtx.Config()["NOTIFICATION_TOPIC"]
	//destBucket := newCtx.Config()["DEST_BUCKET"]
	/*
		get the payload (event data)
	*/
	payload, err := ioutil.ReadAll(in)
	helpers.FatalIfError(err)
	elbody := strings.NewReader(string(payload))
	traceIt(elbody)

	var r tEvent
	err = json.Unmarshal(payload, &r)
	if err != nil {
		log.Print(err)
		return
	}
	//
	traceIt(r.EventType)
	traceIt(r.Data.CompartmentId)
	traceIt(r.Data.ResourceName)
	traceIt(r.Data.ResourceId)
	traceIt(r.Data.AdditionalDetails.BucketName)
	traceIt(r.Data.AdditionalDetails.Namespace)
	//
	/*
		get the bytes of the image stored in bucket which triggered the event
	*/
	a, err := auth.ResourcePrincipalConfigurationProvider()
	traceIt(a)
	//
	oclient, oerr := objectstorage.NewObjectStorageClientWithConfigurationProvider(a)
	helpers.FatalIfError(oerr)
	//
	oreq := objectstorage.GetObjectRequest{
		ObjectName: 		&r.Data.ResourceName,
		NamespaceName:      &r.Data.AdditionalDetails.Namespace,
		BucketName:         &r.Data.AdditionalDetails.BucketName}
	//
	oresp, oerr1 := oclient.GetObject(context.Background(), oreq)
	helpers.FatalIfError(oerr1)
	//
	pictureBytes, err1 := ioutil.ReadAll(oresp.Content)
	helpers.FatalIfError(err1)
	/*
		image detection analysis step
	*/
	a65, err := auth65.ResourcePrincipalConfigurationProvider()
	//
	vclient, verr := aivision65.NewAIServiceVisionClientWithConfigurationProvider(a65)
	helpers.FatalIfError(verr)
	traceIt(vclient)
	//
	id := uuid.New()
	vreq := aivision65.AnalyzeImageRequest{AnalyzeImageDetails: aivision65.AnalyzeImageDetails{
		CompartmentId: &r.Data.CompartmentId,
		Features: []aivision65.ImageFeature{aivision65.ImageObjectDetectionFeature{
		ModelId: nil,
		MaxResults: common65.Int(5)}},			
		Image: aivision65.InlineImageDetails{Data: pictureBytes}},
		OpcRequestId: common65.String(id.String())}
	//
	vresp, verr := vclient.AnalyzeImage(context.Background(), vreq)
	helpers.FatalIfError(verr)
	//
	var body = ""
	for _, value := range vresp.ImageObjects {
		mytag := strings.ToUpper(*value.Name)
		traceIt(fmt.Sprintf("object %s found with relevance %.2f", mytag, *value.Confidence))
		// iterate list of objects to search for
		for _, element := range objects_to_search_for {
			traceIt(fmt.Sprintf("Is %s contained in %s?",mytag, element))
			if strings.Contains(mytag, strings.ToUpper(element)) && *value.Confidence > float32(confidence){
				msg := fmt.Sprintf("found object %s with relevance %.2f in image %s!!!", mytag, *value.Confidence, r.Data.ResourceName)
				body = body + "\n" + msg
				traceIt(msg)
				traceIt(body)
				/*
				* puting the picture in the suspicious bucket
				*/
				/*ele := int64(len(pictureBytes))
				oreq := objectstorage.PutObjectRequest{
					ObjectName: 		&mytag,
					NamespaceName:      &r.Data.AdditionalDetails.Namespace,
					BucketName:         &destBucket,
					ContentLength:		&ele,
					PutObjectBody:		oresp1.Content}
				re, err := oclient.PutObject(context.Background(), oreq)
				trace(re)
				helpers.FatalIfError(err)*/

			}
		}
	}
	/*
		send notification step
	*/
	if len(body) > 1{
		trace("Sending notification")
		client, err := ons.NewNotificationDataPlaneClientWithConfigurationProvider(a65)
		helpers.FatalIfError(err)
		req := ons.PublishMessageRequest{MessageDetails: ons.MessageDetails{Body: common.String(body/*string(pictureBytes)*/),
			Title: common65.String("INTRUSION DETECTION")},
			MessageType:  ons.PublishMessageMessageTypeRawText,
			OpcRequestId: common.String(id.String()),
			TopicId:      common.String(mytopic)}
		traceIt(req)
		resp, err := client.PublishMessage(context.Background(), req)
		helpers.FatalIfError(err)
		traceIt(resp)
		}
}
/*****************************************************************************
	a helper method
******************************************************************************/
func traceIt(s interface{}) {
	if DEBUG == 1 {
			trace(s)
	}	
}
/*****************************************************************************
	a helper method
******************************************************************************/
func trace(s interface{}) {
	log.Print(s)

}